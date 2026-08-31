"""
Transport layer for the ghostchrome JSONL agent protocol.

Defines the Transport protocol (ABC) and a default SubprocessTransport
that drives a long-lived `ghostchrome agent` subprocess via stdin/stdout.
"""
from __future__ import annotations

import json
import subprocess
import threading
import uuid
from abc import ABC, abstractmethod
from queue import Empty, Queue
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    pass


class Transport(ABC):
    """
    Abstract base for sending JSONL ops to a ghostchrome agent and receiving responses.

    Implementations must be thread-safe for concurrent send/receive.
    """

    @abstractmethod
    def send(self, op: str, args: dict | None = None, *, req_id: str | None = None) -> dict:
        """
        Send an op to the agent and return the raw response dict.

        Args:
            op:     Op name (e.g. "navigate", "click").
            args:   Op arguments dict (may be None or empty).
            req_id: Optional correlation id. Auto-generated if not provided.

        Returns:
            The raw response dict ``{id, ok, result, observation, error}``.

        Raises:
            TransportError: on I/O failure or non-ok response.
        """

    @abstractmethod
    def close(self) -> None:
        """Cleanly shut down the transport and release resources."""


class TransportError(RuntimeError):
    """Raised when the transport encounters a protocol or I/O failure."""


class SubprocessTransport(Transport):
    """
    Drives a long-lived ``ghostchrome agent`` subprocess.

    - Spawns the process with text-mode, line-buffered I/O.
    - A background reader thread drains stdout into a per-request queue
      keyed by request id.
    - :py:meth:`send` writes one JSON line to stdin, blocks until the
      matching response line arrives on stdout, and returns the parsed dict.

    Args:
        command:    Binary name or path (default ``"ghostchrome"``).
        args:       Extra positional args after the command (default ``["agent"]``).
        extra_flags: Additional CLI flags appended after ``args``.
        timeout:    Per-call read timeout in seconds (default 30).
    """

    def __init__(
        self,
        command: str = "ghostchrome",
        args: list[str] | None = None,
        extra_flags: list[str] | None = None,
        timeout: float = 30.0,
    ) -> None:
        self._command = command
        self._args = args if args is not None else ["agent"]
        self._extra_flags = extra_flags or []
        self._timeout = timeout

        self._lock = threading.Lock()
        self._queues: dict[str, Queue[dict]] = {}
        self._queue_lock = threading.Lock()

        cmd = [self._command, *self._args, *self._extra_flags]
        self._proc = subprocess.Popen(
            cmd,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,  # line-buffered
        )
        self._closed = False
        self._reader = threading.Thread(target=self._read_loop, daemon=True)
        self._reader.start()
        self._stderr_lock = threading.Lock()
        self._stderr_tail = ""
        self._stderr_reader = threading.Thread(target=self._read_stderr, daemon=True)
        self._stderr_reader.start()

    # ------------------------------------------------------------------
    # Internal reader loop (runs in background thread)
    # ------------------------------------------------------------------

    def _read_loop(self) -> None:
        """Read stdout line-by-line and dispatch to per-request queues."""
        assert self._proc.stdout is not None
        while True:
            line = self._proc.stdout.readline()
            if not line:
                # EOF — process exited or pipe closed
                self._dispatch_eof()
                break
            line = line.strip()
            if line:
                self._dispatch_line(line)

    def _read_stderr(self) -> None:
        """Drain stderr so a noisy child cannot deadlock on a full pipe."""
        assert self._proc.stderr is not None
        while True:
            chunk = self._proc.stderr.read(4096)
            if not chunk:
                return
            with self._stderr_lock:
                self._stderr_tail = (self._stderr_tail + chunk)[-65536:]

    def _dispatch_line(self, line: str) -> None:
        try:
            raw = json.loads(line)
        except json.JSONDecodeError:
            return  # ignore malformed lines
        req_id = raw.get("id")
        if req_id is None:
            return
        with self._queue_lock:
            q = self._queues.get(req_id)
        if q is not None:
            q.put(raw)

    def _dispatch_eof(self) -> None:
        """When the process exits, unblock all waiting send() calls with error."""
        sentinel = {"id": None, "ok": False, "error": "transport closed (process exited)"}
        with self._queue_lock:
            for q in self._queues.values():
                q.put(dict(sentinel))

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def send(self, op: str, args: dict | None = None, *, req_id: str | None = None) -> dict:
        if self._closed:
            raise TransportError("transport is closed")

        rid = req_id or str(uuid.uuid4())
        payload = {"id": rid, "op": op, "args": args or {}}

        q: Queue[dict] = Queue()
        with self._queue_lock:
            self._queues[rid] = q

        try:
            line = json.dumps(payload) + "\n"
            with self._lock:
                if self._proc.stdin is None or self._proc.stdin.closed:
                    raise TransportError("stdin is closed")
                self._proc.stdin.write(line)
                self._proc.stdin.flush()

            try:
                response = q.get(timeout=self._timeout)
            except Empty:
                raise TransportError(f"timeout waiting for response to op={op!r} id={rid!r}")

            return response
        finally:
            with self._queue_lock:
                self._queues.pop(rid, None)

    def close(self) -> None:
        if self._closed:
            return
        self._closed = True
        # Try to send close op first, ignore errors
        try:
            if self._proc.stdin and not self._proc.stdin.closed:
                rid = str(uuid.uuid4())
                payload = json.dumps({"id": rid, "op": "close", "args": {}}) + "\n"
                self._proc.stdin.write(payload)
                self._proc.stdin.flush()
                self._proc.stdin.close()
        except OSError:
            pass
        # Wait for process to exit
        try:
            self._proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self._proc.kill()
            self._proc.wait()
        # Close remaining stdio handles to suppress ResourceWarning
        for handle in (self._proc.stdout, self._proc.stderr):
            if handle is not None and not handle.closed:
                try:
                    handle.close()
                except OSError:
                    pass

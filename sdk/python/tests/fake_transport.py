"""
FakeTransport: an in-process Transport that returns canned JSONL responses.

Used by unit tests to verify client behaviour without a real ghostchrome binary
or Chrome instance.
"""
from __future__ import annotations

import json
from typing import Any

from ghostchrome.transport import Transport, TransportError


class FakeTransport(Transport):
    """
    Deterministic fake transport.

    Provide a list of ``responses`` (raw dicts) in the order they should be
    returned, or a callable ``handler(op, args, req_id)`` for dynamic replies.

    Attributes:
        calls: List of ``(op, args, req_id)`` tuples that were sent.
        responses: Queue of raw response dicts to return.
        closed: True after :meth:`close` is called.
    """

    def __init__(
        self,
        responses: list[dict[str, Any]] | None = None,
        handler: Any | None = None,
    ) -> None:
        self.calls: list[tuple[str, dict, str]] = []
        self._responses: list[dict[str, Any]] = list(responses or [])
        self._handler = handler
        self.closed = False

    def send(self, op: str, args: dict | None = None, *, req_id: str | None = None) -> dict:
        if self.closed:
            raise TransportError("transport is closed")
        rid = req_id or f"fake-{len(self.calls)}"
        args = args or {}
        self.calls.append((op, args, rid))

        if self._handler is not None:
            raw = self._handler(op, args, rid)
        elif self._responses:
            raw = self._responses.pop(0)
        else:
            raise TransportError(f"FakeTransport: no response queued for op={op!r}")

        # Ensure id is set in the response (mirrors real protocol)
        if "id" not in raw:
            raw = dict(raw)
            raw["id"] = rid
        return raw

    def close(self) -> None:
        self.closed = True

    # ---- Helpers --------------------------------------------------------

    def queue(self, response: dict[str, Any]) -> None:
        """Enqueue a single response dict."""
        self._responses.append(response)

    def queue_ok(
        self,
        result: dict[str, Any] | None = None,
        observation: dict[str, Any] | None = None,
    ) -> None:
        """Enqueue a successful response with optional result and observation."""
        self._responses.append({
            "ok": True,
            "result": result or {},
            "observation": observation,
        })

    def queue_error(self, error: str) -> None:
        """Enqueue a failed response."""
        self._responses.append({"ok": False, "error": error})

    @classmethod
    def from_lines(cls, *jsonl_lines: str) -> "FakeTransport":
        """
        Build a FakeTransport from raw JSONL strings.

        Useful for testing line-buffer reassembly scenarios.
        """
        responses = [json.loads(line) for line in jsonl_lines]
        return cls(responses=responses)

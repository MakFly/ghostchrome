"""
Typed structures for the ghostchrome JSONL agent protocol.

Derived from contracts/commands.json (canonical ops registry).
All op args and response shapes are represented as dataclasses.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Optional


# ---------------------------------------------------------------------------
# Wire protocol envelope
# ---------------------------------------------------------------------------

@dataclass
class Request:
    """A single JSONL op sent to the ghostchrome agent on stdin."""
    id: str
    op: str
    args: dict[str, Any] = field(default_factory=dict)


@dataclass
class ConsoleError:
    level: str
    text: str
    source: str = ""


@dataclass
class NetworkError:
    url: str
    status: int
    failed: str = ""


@dataclass
class Observation:
    """
    Observation packet included in every response.
    Describes what changed during the op.
    Empty fields are omitted by the agent and default to None/[].

    Note: ``events`` is a top-level envelope field, NOT part of the observation
    object. It is only present when the agent is started with ``--observe``.
    """
    url: Optional[str] = None
    console_errors: list[ConsoleError] = field(default_factory=list)
    network_failed: list[NetworkError] = field(default_factory=list)
    a11y_diff: Optional[str] = None
    dialog: Optional[str] = None
    captcha_hint: Optional[str] = None


@dataclass
class Response:
    """A single JSONL line returned by the ghostchrome agent on stdout."""
    id: str
    ok: bool
    result: Any = field(default_factory=dict)
    observation: Optional[Observation] = None
    error: Optional[str] = None
    events: list[dict[str, Any]] = field(default_factory=list)


# ---------------------------------------------------------------------------
# Helpers: parse raw dicts into typed objects
# ---------------------------------------------------------------------------

def _parse_observation(raw: dict[str, Any] | None) -> Optional[Observation]:
    if raw is None:
        return None
    console_errors = [
        ConsoleError(
            level=e.get("level", ""),
            text=e.get("text", ""),
            source=e.get("source", ""),
        )
        for e in raw.get("console_errors", [])
    ]
    network_failed = [
        NetworkError(
            url=e.get("url", ""),
            status=e.get("status", 0),
            failed=e.get("failed", ""),
        )
        for e in raw.get("network_failed", [])
    ]
    return Observation(
        url=raw.get("url"),
        console_errors=console_errors,
        network_failed=network_failed,
        a11y_diff=raw.get("a11y_diff"),
        dialog=raw.get("dialog"),
        captcha_hint=raw.get("captcha_hint"),
    )


def parse_response(line: str) -> Response:
    """Parse a raw JSONL line from the agent into a typed Response."""
    import json
    raw = json.loads(line)
    return Response(
        id=raw["id"],
        ok=raw.get("ok", False),
        result=raw.get("result"),
        observation=_parse_observation(raw.get("observation")),
        error=raw.get("error"),
        events=raw.get("events", []),
    )


# ---------------------------------------------------------------------------
# Op-specific result types
# ---------------------------------------------------------------------------

@dataclass
class NavigateResult:
    """Result of the ``navigate`` op."""
    url: str
    title: str
    status: int
    time_ms: int = 0


@dataclass
class BackForwardResult:
    """Result of the ``back`` / ``forward`` ops."""
    url: Optional[str] = None
    title: Optional[str] = None


@dataclass
class ExtractStats:
    """Stats block returned inside an ``extract`` result."""
    total_nodes: int = 0
    filtered_nodes: int = 0
    interactive_count: int = 0


@dataclass
class ExtractResult:
    """Result of the ``extract`` op."""
    nodes: list[Any]
    refs: dict[str, Any]
    stats: ExtractStats


@dataclass
class FillResult:
    """Result of the ``fill`` op."""
    filled: int = 0


@dataclass
class ScrollResult:
    """Result of the ``scroll_by`` and ``scroll_to`` ops — current Y position."""
    y: int = 0


@dataclass
class UrlResult:
    """Result of the ``url`` op."""
    url: str
    title: str = ""


@dataclass
class ErrorEntry:
    """A single console or network error entry returned by the ``errors`` op.

    Maps exactly to the Go ``ErrorEntry`` struct in ``engine/errors.go``:

    - type:    ``"console"`` or ``"network"``
    - level:   ``"error"``, ``"warning"``, ``"4xx"``, ``"5xx"``
    - message: error message text or URL
    - source:  file:line for console errors, URL for network errors
    - status:  HTTP status code (network errors only, omitted otherwise)
    - method:  HTTP method (network errors only, omitted otherwise)
    - time_ms: timestamp relative to collector start
    """
    type: str
    level: str
    message: str
    source: str
    time_ms: int
    status: Optional[int] = None
    method: Optional[str] = None


@dataclass
class EvalResult:
    """Result of the ``eval`` op."""
    value: Any


@dataclass
class ScreenshotResult:
    """Result of the ``screenshot`` op.

    The JSONL agent surface always returns ``base64`` and ``mime``.
    ``path`` is not emitted over the wire; it is kept as an optional
    field for callers that route the image to disk themselves.
    """
    mime: str = ""
    base64: Optional[str] = None
    path: Optional[str] = None


@dataclass
class InitResult:
    """Result of the ``init`` op (result is omitted by the agent; fields are always None)."""
    session_id: Optional[str] = None
    browser_version: Optional[str] = None


# ---------------------------------------------------------------------------
# Error class (semantic equivalent of TS GhostchromeError)
# ---------------------------------------------------------------------------

class GhostchromeError(RuntimeError):
    """Raised when the agent returns ok=false.

    Attributes:
        op:      The op name that failed.
        message: The error string from the agent.
    """

    def __init__(self, op: str, message: str) -> None:
        super().__init__(f"{op}: {message}")
        self.op = op
        self.message = message

"""Real-time agent transcript writer for demo terminals.

Provides a callable that formats agent tool-loop events as ANSI-colored,
timestamped text and writes them to a file.  Designed for ``tail -f``.

Usage::

    writer = open_transcript("/tmp/agent.log")
    result = execute(config, session, query, on_step=writer)
    writer.close()
"""

from __future__ import annotations

import json
from datetime import datetime, timezone


# ANSI escape codes
_CYAN = "\033[36m"
_YELLOW = "\033[33m"
_DIM = "\033[2m"
_GREEN = "\033[32m"
_RESET = "\033[0m"

_COLORS = {
    "thinking": _CYAN,
    "tool_call": _YELLOW,
    "tool_result": _DIM,
    "answer": _GREEN,
}

_LABELS = {
    "thinking": "THINKING",
    "tool_call": "TOOL CALL",
    "tool_result": "TOOL RESULT",
    "answer": "ANSWER",
}


class TranscriptWriter:
    """Formats and writes agent events to a file handle.

    Instances are directly callable, so they can be passed as the
    ``on_step`` callback to :func:`harness.runner.execute`.
    """

    def __init__(self, fh) -> None:  # noqa: ANN001 – accepts any file-like
        self._fh = fh

    # -- public interface ----------------------------------------------------

    def __call__(self, kind: str, data: dict) -> None:
        color = _COLORS.get(kind, "")
        label = _LABELS.get(kind, kind.upper())
        ts = datetime.now(timezone.utc).strftime("%H:%M:%S")

        if kind == "thinking":
            self._write(f"{_DIM}[{ts}]{_RESET} {color}{label}{_RESET}\n")
            self._write_indented(data.get("text", ""), color)

        elif kind == "tool_call":
            name = data.get("name", "?")
            self._write(f"{_DIM}[{ts}]{_RESET} {color}{label}: {name}{_RESET}\n")
            for key, val in data.get("input", {}).items():
                self._write(f"  {color}{key}: {json.dumps(val)}{_RESET}\n")

        elif kind == "tool_result":
            name = data.get("name", "?")
            self._write(f"{_DIM}[{ts}]{_RESET} {color}{label}: {name}{_RESET}\n")
            self._write_indented(data.get("result", ""), color)

        elif kind == "answer":
            self._write(f"{_DIM}[{ts}]{_RESET} {color}{label}{_RESET}\n")
            self._write_indented(data.get("text", ""), color)

        self._write("\n")
        self._fh.flush()

    def close(self) -> None:
        self._fh.close()

    # -- helpers -------------------------------------------------------------

    def _write(self, text: str) -> None:
        self._fh.write(text)

    def _write_indented(self, text: str, color: str) -> None:
        for line in text.splitlines():
            self._write(f"  {color}{line}{_RESET}\n")


def open_transcript(path: str) -> TranscriptWriter:
    """Open *path* for writing and return a :class:`TranscriptWriter`."""
    fh = open(path, "w")  # noqa: SIM115 – caller owns lifecycle via .close()
    return TranscriptWriter(fh)

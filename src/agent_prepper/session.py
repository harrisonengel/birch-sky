"""In-memory session store for the prepper.

A session holds the running message transcript, the buyer id, the turn
counter, and (once finalized) the Briefing. Keys are UUIDs. Storage is a
plain process-local dict — sessions do not survive restarts. The spec
covers why this is acceptable for the MVP.
"""

from __future__ import annotations

import threading
import uuid
from dataclasses import dataclass, field
from typing import Optional


@dataclass
class Briefing:
    goal_summary: str
    selection_criteria: list[str]
    analysis_mode: str
    background: str = ""
    constraints: str = ""

    def to_dict(self) -> dict:
        return {
            "goal_summary": self.goal_summary,
            "selection_criteria": list(self.selection_criteria),
            "analysis_mode": self.analysis_mode,
            "background": self.background,
            "constraints": self.constraints,
        }


@dataclass
class PrepperSession:
    session_id: str
    buyer_id: str
    # Anthropic message format: list of {"role": "user"|"assistant", "content": ...}
    messages: list[dict] = field(default_factory=list)
    turn: int = 0
    briefing: Optional[Briefing] = None

    @property
    def status(self) -> str:
        return "ready" if self.briefing is not None else "asking"


class SessionStore:
    """Thread-safe in-process map of session_id -> PrepperSession."""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._sessions: dict[str, PrepperSession] = {}

    def create(self, buyer_id: str) -> PrepperSession:
        session_id = str(uuid.uuid4())
        session = PrepperSession(session_id=session_id, buyer_id=buyer_id)
        with self._lock:
            self._sessions[session_id] = session
        return session

    def get(self, session_id: str) -> Optional[PrepperSession]:
        with self._lock:
            return self._sessions.get(session_id)


# Module-level singleton; FastAPI will reuse this across requests.
STORE = SessionStore()

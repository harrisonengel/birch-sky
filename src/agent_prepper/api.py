"""FastAPI wrapper around the prepper tool loop.

Routes:
  POST /api/prepper/start         create session, run turn 1
  POST /api/prepper/respond       advance an existing session by one turn
  GET  /api/prepper/session/{id}  dump session state (debug + CLI)
  GET  /health                    liveness probe
"""

from __future__ import annotations

from typing import Optional

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

from .config import PrepperConfig, load_from_env
from .runner import execute_turn
from .session import PrepperSession, STORE

app = FastAPI(title="IE Agent Prepper", version="0.1.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)


def _config() -> PrepperConfig:
    try:
        return load_from_env()
    except RuntimeError as e:
        raise HTTPException(status_code=503, detail=str(e))


class StartRequest(BaseModel):
    buyer_id: str = Field(..., min_length=1)
    initial_query: str = Field(..., min_length=1)


class RespondRequest(BaseModel):
    session_id: str = Field(..., min_length=1)
    answer: str = Field(..., min_length=1)


class TurnResponse(BaseModel):
    session_id: str
    status: str
    turn: int
    question: Optional[str] = None
    briefing: Optional[dict] = None


def _to_response(session: PrepperSession, result) -> TurnResponse:
    return TurnResponse(
        session_id=session.session_id,
        status=result.status,
        turn=session.turn,
        question=result.question,
        briefing=result.briefing.to_dict() if result.briefing is not None else None,
    )


@app.post("/api/prepper/start", response_model=TurnResponse)
def start(req: StartRequest) -> TurnResponse:
    config = _config()
    session = STORE.create(buyer_id=req.buyer_id)
    result = execute_turn(config, session, req.initial_query)
    return _to_response(session, result)


@app.post("/api/prepper/respond", response_model=TurnResponse)
def respond(req: RespondRequest) -> TurnResponse:
    session = STORE.get(req.session_id)
    if session is None:
        raise HTTPException(status_code=404, detail="session not found")
    if session.briefing is not None:
        raise HTTPException(
            status_code=409,
            detail="session already finalized; no more turns allowed",
        )
    config = _config()
    result = execute_turn(config, session, req.answer)
    return _to_response(session, result)


@app.get("/api/prepper/session/{session_id}")
def get_session(session_id: str) -> dict:
    session = STORE.get(session_id)
    if session is None:
        raise HTTPException(status_code=404, detail="session not found")
    return {
        "session_id": session.session_id,
        "buyer_id": session.buyer_id,
        "status": session.status,
        "turn": session.turn,
        # Anthropic content blocks may be objects; coerce the assistant
        # blocks to plain dicts/strings for the JSON response.
        "transcript": [_serialize_message(m) for m in session.messages],
        "briefing": session.briefing.to_dict() if session.briefing is not None else None,
    }


def _serialize_message(msg: dict) -> dict:
    content = msg["content"]
    if isinstance(content, str):
        return {"role": msg["role"], "content": content}
    parts = []
    for block in content:
        t = getattr(block, "type", None)
        if t == "text":
            parts.append({"type": "text", "text": block.text})
        elif t == "tool_use":
            parts.append(
                {"type": "tool_use", "name": block.name, "input": block.input}
            )
        else:
            parts.append({"type": t or "unknown"})
    return {"role": msg["role"], "content": parts}


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}

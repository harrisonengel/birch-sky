"""FastAPI wrapper around the agent harness runner.

Exposes POST /api/run so the frontend (or any HTTP client) can invoke
a buyer-agent session without shelling out to the CLI.

Config comes from environment variables, not YAML files:
  - MODEL_NAME          (default: claude-sonnet-4-5)
  - ANTHROPIC_API_KEY   (required)
  - OPENSEARCH_URL      (default: http://opensearch:9200)
  - OPENSEARCH_INDEX    (default: listings)
"""

from __future__ import annotations

import os

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

from .config import HarnessConfig
from .runner import execute
from .session import from_context
from .transcript import open_transcript

app = FastAPI(title="IE Agent Harness", version="0.1.0")

TRANSCRIPT_PATH = os.environ.get("TRANSCRIPT_PATH")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)


def _load_config() -> HarnessConfig:
    api_key = os.environ.get("ANTHROPIC_API_KEY", "")
    if not api_key:
        raise RuntimeError("ANTHROPIC_API_KEY is not set")
    return HarnessConfig(
        model=os.environ.get("MODEL_NAME", "claude-sonnet-4-5"),
        api_key=api_key,
        opensearch_url=os.environ.get("OPENSEARCH_URL", "http://opensearch:9200"),
        opensearch_index=os.environ.get("OPENSEARCH_INDEX", "listings"),
    )


class RunRequest(BaseModel):
    starting_context: dict = Field(
        ..., description="Agent context with background, goal, constraints"
    )
    user_input: str = Field(..., description="The buyer's query")
    max_turns: int = Field(default=20, ge=1, le=50)


class RunResponse(BaseModel):
    response: str
    transcript_path: str | None = None


@app.post("/api/run", response_model=RunResponse)
def run_agent(req: RunRequest) -> RunResponse:
    try:
        config = _load_config()
    except RuntimeError as e:
        raise HTTPException(status_code=503, detail=str(e))

    session = from_context(req.starting_context, max_turns=req.max_turns)

    writer = None
    on_step = None
    if TRANSCRIPT_PATH:
        writer = open_transcript(TRANSCRIPT_PATH)
        on_step = writer

    try:
        result = execute(config, session, req.user_input, on_step=on_step)
    finally:
        if writer:
            writer.close()

    return RunResponse(response=result, transcript_path=TRANSCRIPT_PATH)


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}

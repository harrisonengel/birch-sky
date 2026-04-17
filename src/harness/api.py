"""FastAPI wrapper around the agent harness.

The harness is the single entry point for the frontend. The frontend cannot
reach the market platform directly — it goes through `/api/enter` here, which
is the buyer "entering" the marketplace via their agent. The harness then
calls the market platform's `/api/v1/search` endpoint to fulfill the request.

Two endpoints are exposed:
  - POST /api/enter — buyer-facing entry; returns ranked listings for a query.
  - POST /api/run — full agent loop (multi-turn, tool-using). Used by callers
    that want the agent to drive the search rather than a simple pass-through.

Config comes from environment variables, not YAML files:
  - MODEL_NAME           (default: claude-sonnet-4-5)
  - ANTHROPIC_API_KEY    (required for /api/run)
  - MARKET_PLATFORM_URL  (default: http://market-platform:8080)
"""

from __future__ import annotations

import os

import requests
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

from .config import HarnessConfig
from .runner import execute
from .session import from_context

app = FastAPI(title="IE Agent Harness", version="0.1.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)


def _market_url() -> str:
    return os.environ.get("MARKET_PLATFORM_URL", "http://market-platform:8080").rstrip("/")


def _load_config() -> HarnessConfig:
    api_key = os.environ.get("ANTHROPIC_API_KEY", "")
    if not api_key:
        raise RuntimeError("ANTHROPIC_API_KEY is not set")
    return HarnessConfig(
        model=os.environ.get("MODEL_NAME", "claude-sonnet-4-5"),
        api_key=api_key,
        market_platform_url=_market_url(),
    )


class EnterRequest(BaseModel):
    query: str = Field(..., description="Buyer's natural-language query")
    mode: str = Field(default="hybrid", description="search mode: text, vector, or hybrid")
    per_page: int = Field(default=10, ge=1, le=100)
    category: str | None = Field(default=None, description="Optional category filter")
    max_price_cents: int | None = Field(default=None, description="Optional price ceiling")


class EnterResponse(BaseModel):
    results: list[dict]
    total: int
    mode: str


@app.post("/api/enter", response_model=EnterResponse)
def enter(req: EnterRequest) -> EnterResponse:
    body: dict = {
        "query": req.query,
        "mode": req.mode,
        "per_page": req.per_page,
    }
    if req.category:
        body["category"] = req.category
    if req.max_price_cents is not None:
        body["max_price_cents"] = req.max_price_cents

    endpoint = f"{_market_url()}/api/v1/search"
    try:
        resp = requests.post(endpoint, json=body, timeout=30)
    except requests.RequestException as e:
        raise HTTPException(status_code=502, detail=f"market platform unreachable: {e}")

    if resp.status_code != 200:
        raise HTTPException(status_code=resp.status_code, detail=resp.text)

    data = resp.json()
    return EnterResponse(
        results=data.get("results") or [],
        total=int(data.get("total") or 0),
        mode=str(data.get("mode") or req.mode),
    )


class RunRequest(BaseModel):
    starting_context: dict = Field(
        ..., description="Agent context with background, goal, constraints"
    )
    user_input: str = Field(..., description="The buyer's query")
    max_turns: int = Field(default=20, ge=1, le=50)


class RunResponse(BaseModel):
    response: str


@app.post("/api/run", response_model=RunResponse)
def run_agent(req: RunRequest) -> RunResponse:
    try:
        config = _load_config()
    except RuntimeError as e:
        raise HTTPException(status_code=503, detail=str(e))

    session = from_context(req.starting_context, max_turns=req.max_turns)
    result = execute(config, session, req.user_input)
    return RunResponse(response=result)


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}

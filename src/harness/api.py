"""FastAPI wrapper around the agent harness.

The harness is the single entry point for the frontend. The frontend cannot
reach the market platform directly — it goes through `/api/enter` here, which
is the buyer "entering" the marketplace via their agent. The harness then
runs the full multi-turn tool-using agent loop and returns a structured buy
recommendation. There is no simple pass-through search; the agent drives
every request.

Config comes from environment variables, not YAML files:
  - MODEL_NAME           (default: claude-sonnet-4-5)
  - ANTHROPIC_API_KEY    (required)
  - MARKET_PLATFORM_URL  (default: http://market-platform:8080)
"""

from __future__ import annotations

import os

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

from .config import HarnessConfig
from .runner import execute
from .session import from_context
from .tools import HydrationError, hydrate_buy_listings

app = FastAPI(title="IE Agent Harness", version="0.1.0")

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
        market_platform_url=os.environ.get(
            "MARKET_PLATFORM_URL", "http://market-platform:8080"
        ).rstrip("/"),
    )


class EnterRequest(BaseModel):
    starting_context: dict = Field(
        ..., description="Agent context with background, goal, constraints"
    )
    user_input: str = Field(..., description="The buyer's query")
    max_turns: int = Field(default=20, ge=1, le=50)


class BuyListing(BaseModel):
    id: str
    price: int
    listing_description: str
    seller: str


class EnterResponse(BaseModel):
    buy_listings: list[BuyListing]


@app.post("/api/enter", response_model=EnterResponse)
def enter(req: EnterRequest) -> EnterResponse:
    try:
        config = _load_config()
    except RuntimeError as e:
        raise HTTPException(status_code=503, detail=str(e))

    session = from_context(req.starting_context, max_turns=req.max_turns)
    recommendations = execute(config, session, req.user_input)

    try:
        buy_listings = hydrate_buy_listings(recommendations)
    except HydrationError as e:
        raise HTTPException(status_code=422, detail=str(e))

    return EnterResponse(buy_listings=[BuyListing(**item) for item in buy_listings])


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}

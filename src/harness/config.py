from __future__ import annotations

import os
from dataclasses import dataclass

import yaml


@dataclass
class HarnessConfig:
    """Infrastructure config — model endpoint, credentials, search backend.

    Session-level state (starting context, max turns) is NOT stored here;
    it is passed in per-invocation via the session file.
    """

    model: str
    api_key: str
    market_platform_url: str


def load(path: str) -> HarnessConfig:
    with open(path) as f:
        raw = yaml.safe_load(f)

    model_cfg = raw.get("model", {})
    model_name = model_cfg.get("name")
    if not model_name:
        raise ValueError("model.name is required in config (e.g. 'claude-sonnet-4-5')")

    api_key_env = model_cfg.get("api_key_env", "ANTHROPIC_API_KEY")
    api_key = os.environ.get(api_key_env)
    if not api_key:
        raise ValueError(f"environment variable {api_key_env} is not set")

    market_cfg = raw.get("market_platform", {})
    market_url = market_cfg.get("url", "http://localhost:8080")

    return HarnessConfig(
        model=model_name,
        api_key=api_key,
        market_platform_url=market_url,
    )

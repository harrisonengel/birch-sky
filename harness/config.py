from __future__ import annotations

import os
from dataclasses import dataclass, field

import yaml


@dataclass
class HarnessConfig:
    model: str
    api_key: str
    opensearch_url: str
    opensearch_index: str = "listings"
    opensearch_user: str | None = None
    opensearch_pass: str | None = None
    starting_context: dict = field(default_factory=dict)
    max_turns: int = 20


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

    os_cfg = raw.get("opensearch", {})
    os_url = os_cfg.get("url", "http://localhost:9200")
    os_index = os_cfg.get("index", "listings")
    os_user = os_cfg.get("user")
    os_pass = os_cfg.get("pass")

    session = raw.get("session", {})
    starting_context = session.get("starting_context", {})
    max_turns = int(session.get("max_turns", 20))

    return HarnessConfig(
        model=model_name,
        api_key=api_key,
        opensearch_url=os_url,
        opensearch_index=os_index,
        opensearch_user=os_user,
        opensearch_pass=os_pass,
        starting_context=starting_context,
        max_turns=max_turns,
    )

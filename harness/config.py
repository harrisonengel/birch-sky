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
    opensearch_url: str
    opensearch_index: str = "listings"
    opensearch_user: str | None = None
    opensearch_pass: str | None = None


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

    return HarnessConfig(
        model=model_name,
        api_key=api_key,
        opensearch_url=os_url,
        opensearch_index=os_index,
        opensearch_user=os_user,
        opensearch_pass=os_pass,
    )

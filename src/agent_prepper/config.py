"""Environment-driven config for the agent-prepper service.

Mirrors harness/config.py but is simpler: prepper has no search backend,
only an Anthropic model + a turn cap.
"""

from __future__ import annotations

import os
from dataclasses import dataclass


@dataclass
class PrepperConfig:
    model: str
    api_key: str
    max_turns: int = 6


def load_from_env() -> PrepperConfig:
    api_key = os.environ.get("ANTHROPIC_API_KEY", "")
    if not api_key:
        raise RuntimeError("ANTHROPIC_API_KEY is not set")
    return PrepperConfig(
        model=os.environ.get("MODEL_NAME", "claude-sonnet-4-5"),
        api_key=api_key,
        max_turns=int(os.environ.get("PREPPER_MAX_TURNS", "6")),
    )

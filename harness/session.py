from __future__ import annotations

from dataclasses import dataclass

from .config import HarnessConfig


@dataclass
class SessionState:
    mode: str
    instructions: str


def from_config(config: HarnessConfig) -> SessionState:
    ctx = config.starting_context
    parts = [
        "You are a buyer's agent on the Information Exchange marketplace.",
        "Use the search_opensearch tool to find data listings that match the buyer's needs.",
    ]

    if bg := ctx.get("background"):
        parts.append(f"\n## Background\n{bg}")
    if goal := ctx.get("goal"):
        parts.append(f"\n## Goal\n{goal}")
    if constraints := ctx.get("constraints"):
        parts.append(f"\n## Constraints\n{constraints}")

    return SessionState(mode="context", instructions="\n".join(parts))

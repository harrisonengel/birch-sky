from __future__ import annotations

from dataclasses import dataclass

import yaml


@dataclass
class Session:
    """Per-invocation state: agent instructions and turn budget.

    Callers provide a Session either by loading a YAML file (`load`) or by
    constructing one in-process via `from_context`. The session is not part
    of the harness config — it comes in with each call.
    """

    instructions: str
    max_turns: int = 20


def _build_instructions(starting_context: dict) -> str:
    parts = [
        "You are a buyer's agent on the Information Exchange marketplace.",
        "Use the search_opensearch tool to find data listings that match the buyer's needs.",
    ]
    if bg := starting_context.get("background"):
        parts.append(f"\n## Background\n{bg}")
    if goal := starting_context.get("goal"):
        parts.append(f"\n## Goal\n{goal}")
    if constraints := starting_context.get("constraints"):
        parts.append(f"\n## Constraints\n{constraints}")
    return "\n".join(parts)


def from_context(starting_context: dict, max_turns: int = 20) -> Session:
    return Session(
        instructions=_build_instructions(starting_context),
        max_turns=max_turns,
    )


def load(path: str) -> Session:
    with open(path) as f:
        raw = yaml.safe_load(f) or {}

    starting_context = raw.get("starting_context", {})
    if not starting_context:
        raise ValueError(
            f"session file {path} is missing required 'starting_context' block"
        )

    max_turns = int(raw.get("max_turns", 20))
    return from_context(starting_context, max_turns=max_turns)

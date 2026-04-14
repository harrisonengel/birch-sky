"""Anthropic tool-loop runner for the prepper.

Each call to `execute_turn` advances the conversation by exactly one
model turn. Either the model emits text (a clarifying question) or it
calls the `finalize_context` tool (terminal). On the final allowed turn
we instruct the model to finalize.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Optional

from anthropic import Anthropic

from .config import PrepperConfig
from .prompts import FINALIZE_CONTEXT_TOOL, LAST_TURN_NUDGE, SYSTEM_PROMPT
from .session import Briefing, PrepperSession


@dataclass
class TurnResult:
    status: str  # "asking" | "ready"
    question: Optional[str] = None
    briefing: Optional[Briefing] = None


def _briefing_from_tool_input(raw: dict) -> Briefing:
    """Validate a finalize_context tool call and build a Briefing.

    Raises ValueError if required fields are missing or the analysis_mode
    is not one of the two allowed values.
    """
    goal = (raw.get("goal_summary") or "").strip()
    criteria = raw.get("selection_criteria") or []
    mode = raw.get("analysis_mode") or ""
    if not goal:
        raise ValueError("finalize_context: goal_summary is required")
    if not isinstance(criteria, list) or not criteria:
        raise ValueError("finalize_context: selection_criteria must be a non-empty list")
    if mode not in ("compute_to_end", "evaluate_then_decide"):
        raise ValueError(
            f"finalize_context: analysis_mode must be 'compute_to_end' or "
            f"'evaluate_then_decide', got {mode!r}"
        )
    return Briefing(
        goal_summary=goal,
        selection_criteria=[str(c) for c in criteria],
        analysis_mode=mode,
        background=str(raw.get("background") or "").strip(),
        constraints=str(raw.get("constraints") or "").strip(),
    )


def execute_turn(
    config: PrepperConfig,
    session: PrepperSession,
    user_message: str,
) -> TurnResult:
    """Append the user message, run one model turn, return the result.

    The session is mutated in place: the user/assistant messages are
    appended to `session.messages`, `session.turn` is incremented, and
    if the model calls finalize_context the resulting Briefing is stored
    on `session.briefing`.
    """
    if session.briefing is not None:
        # Defensive: callers should check status before calling.
        return TurnResult(status="ready", briefing=session.briefing)

    session.messages.append({"role": "user", "content": user_message})
    session.turn += 1

    is_last_turn = session.turn >= config.max_turns
    system = SYSTEM_PROMPT
    if is_last_turn:
        system = SYSTEM_PROMPT + "\n\n" + LAST_TURN_NUDGE

    client = Anthropic(api_key=config.api_key)
    response = client.messages.create(
        model=config.model,
        max_tokens=1024,
        system=system,
        tools=[FINALIZE_CONTEXT_TOOL],
        # Force the tool on the last turn so we always terminate cleanly.
        tool_choice=(
            {"type": "tool", "name": "finalize_context"}
            if is_last_turn
            else {"type": "auto"}
        ),
        messages=session.messages,
    )

    # Persist the assistant message exactly as returned, so that any future
    # turn against this session keeps a valid transcript.
    session.messages.append({"role": "assistant", "content": response.content})

    # Look for a finalize_context tool call.
    for block in response.content:
        if getattr(block, "type", None) == "tool_use" and block.name == "finalize_context":
            briefing = _briefing_from_tool_input(block.input or {})
            session.briefing = briefing
            return TurnResult(status="ready", briefing=briefing)

    # Otherwise, return the first text block as the next question.
    for block in response.content:
        if getattr(block, "type", None) == "text" and block.text.strip():
            return TurnResult(status="asking", question=block.text.strip())

    # Pathological: model returned nothing useful. Treat as a generic
    # nudge so the buyer can keep going.
    return TurnResult(
        status="asking",
        question="Could you tell me a bit more about what you're trying to do?",
    )

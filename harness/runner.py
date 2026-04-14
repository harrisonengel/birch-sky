from __future__ import annotations

from collections.abc import Callable

from anthropic import Anthropic

from . import config as config_module
from . import session as session_module
from . import tools
from .config import HarnessConfig
from .session import Session
from .transcript import open_transcript


def run(
    config_path: str,
    session_path: str,
    user_input: str,
    transcript_path: str | None = None,
) -> None:
    """CLI entry point: load config/session from disk and print the result."""
    config = config_module.load(config_path)
    session = session_module.load(session_path)

    writer = None
    on_step: Callable[[str, dict], None] | None = None
    if transcript_path:
        writer = open_transcript(transcript_path)
        on_step = writer

    try:
        result = execute(config, session, user_input, on_step=on_step)
    finally:
        if writer:
            writer.close()

    print(result)


def execute(
    config: HarnessConfig,
    session: Session,
    user_input: str,
    on_step: Callable[[str, dict], None] | None = None,
) -> str:
    """Run one agent invocation against the given config + session.

    This is the programmatic entry point — a future HTTP API would call
    this directly with in-memory objects rather than loading from disk.

    *on_step*, when provided, is called at each stage of the tool loop
    with ``(kind, data)`` where *kind* is one of ``"thinking"``,
    ``"tool_call"``, ``"tool_result"``, or ``"answer"``.

    Returns the agent's final text response, or an empty string if the
    loop terminated without producing one.
    """
    tools.configure(
        url=config.opensearch_url,
        index=config.opensearch_index,
        user=config.opensearch_user,
        password=config.opensearch_pass,
    )

    client = Anthropic(api_key=config.api_key)
    messages: list[dict] = [{"role": "user", "content": user_input}]

    for _ in range(session.max_turns):
        response = client.messages.create(
            model=config.model,
            max_tokens=4096,
            system=session.instructions,
            tools=[tools.SEARCH_TOOL_SCHEMA],
            messages=messages,
        )

        messages.append({"role": "assistant", "content": response.content})

        if response.stop_reason == "tool_use":
            # Emit thinking blocks that precede the tool call
            if on_step:
                for block in response.content:
                    if block.type == "text":
                        on_step("thinking", {"text": block.text})

            tool_results = []
            for block in response.content:
                if block.type == "tool_use":
                    if on_step:
                        on_step("tool_call", {"name": block.name, "input": block.input or {}})

                    result = tools.dispatch(block.name, block.input or {})

                    if on_step:
                        on_step("tool_result", {"name": block.name, "result": result})

                    tool_results.append(
                        {
                            "type": "tool_result",
                            "tool_use_id": block.id,
                            "content": result,
                        }
                    )
            messages.append({"role": "user", "content": tool_results})
            continue

        for block in response.content:
            if block.type == "text":
                if on_step:
                    on_step("answer", {"text": block.text})
                return block.text
        return ""

    return f"[harness] max_turns ({session.max_turns}) reached without final answer"

from __future__ import annotations

from anthropic import Anthropic

from . import config as config_module
from . import session as session_module
from . import tools
from .config import HarnessConfig
from .session import Session


def run(config_path: str, session_path: str, user_input: str) -> None:
    """CLI entry point: load config/session from disk and print the result."""
    config = config_module.load(config_path)
    session = session_module.load(session_path)
    result = execute(config, session, user_input)
    print(result)


def execute(
    config: HarnessConfig,
    session: Session,
    user_input: str,
    trace: list | None = None,
) -> str:
    """Run one agent invocation against the given config + session.

    This is the programmatic entry point — a future HTTP API would call
    this directly with in-memory objects rather than loading from disk.

    If `trace` is provided, each reasoning text block and tool call is
    appended as a step record. The final text block is returned (not
    appended) so callers can store it separately as final_output.

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
            tool_results = []
            for block in response.content:
                if block.type == "text" and trace is not None:
                    trace.append(
                        {
                            "step": len(trace) + 1,
                            "kind": "reasoning",
                            "content": block.text,
                        }
                    )
                if block.type == "tool_use":
                    result = tools.dispatch(block.name, block.input or {})
                    if trace is not None:
                        trace.append(
                            {
                                "step": len(trace) + 1,
                                "kind": "tool_call",
                                "tool": block.name,
                                "input": block.input,
                                "output": result,
                            }
                        )
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
                return block.text
        return ""

    return f"[harness] max_turns ({session.max_turns}) reached without final answer"

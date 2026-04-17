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


def execute(config: HarnessConfig, session: Session, user_input: str) -> str:
    """Run one agent invocation against the given config + session.

    This is the programmatic entry point — a future HTTP API would call
    this directly with in-memory objects rather than loading from disk.

    Returns the agent's final text response, or an empty string if the
    loop terminated without producing one.
    """
    tools.configure(market_platform_url=config.market_platform_url)

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
                if block.type == "tool_use":
                    result = tools.dispatch(block.name, block.input or {})
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

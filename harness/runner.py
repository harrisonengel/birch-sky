from __future__ import annotations

from anthropic import Anthropic

from . import config as config_module
from . import session as session_module
from . import tools
from .session import Session


def run(config_path: str, session_path: str, user_input: str) -> None:
    """Run one invocation of the buyer-agent harness.

    Args:
        config_path: Path to infrastructure config (model, opensearch).
        session_path: Path to session file (starting context, max_turns).
            The session is per-call and NOT part of the harness config.
        user_input: The initial user message for this call.
    """
    config = config_module.load(config_path)
    session = session_module.load(session_path)
    _execute(config, session, user_input)


def _execute(config, session: Session, user_input: str) -> None:
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
                print(block.text)
        return

    print(f"[harness] max_turns ({session.max_turns}) reached without final answer")

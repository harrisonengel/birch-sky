from __future__ import annotations

from anthropic import Anthropic

from . import config as config_module
from . import tools
from .session import from_config


def run(config_path: str, user_input: str) -> None:
    config = config_module.load(config_path)

    tools.configure(
        url=config.opensearch_url,
        index=config.opensearch_index,
        user=config.opensearch_user,
        password=config.opensearch_pass,
    )

    client = Anthropic(api_key=config.api_key)
    session = from_config(config)

    messages: list[dict] = [{"role": "user", "content": user_input}]

    for _ in range(config.max_turns):
        response = client.messages.create(
            model=config.model,
            max_tokens=4096,
            system=session.instructions,
            tools=[tools.SEARCH_TOOL_SCHEMA],
            messages=messages,
        )

        # Append the assistant turn to the transcript.
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

        # Any non-tool-use stop reason ends the loop — print final text blocks.
        for block in response.content:
            if block.type == "text":
                print(block.text)
        return

    print(f"[harness] max_turns ({config.max_turns}) reached without final answer")

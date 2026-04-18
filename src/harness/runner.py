from __future__ import annotations

import json

from anthropic import Anthropic

from . import config as config_module
from . import session as session_module
from . import tools
from .config import HarnessConfig
from .session import Session


def run(config_path: str, session_path: str, user_input: str) -> None:
    """CLI entry point: load config/session from disk and print buy_listings."""
    config = config_module.load(config_path)
    session = session_module.load(session_path)
    recommendations = execute(config, session, user_input)
    tools.configure(market_platform_url=config.market_platform_url)
    buy_listings = tools.hydrate_buy_listings(recommendations)
    print(json.dumps({"buy_listings": buy_listings}, indent=2))


def execute(
    config: HarnessConfig, session: Session, user_input: str
) -> list[dict]:
    """Run one agent invocation and return the agent's recommendation list.

    The list contains {seller_id, listing_id} entries exactly as the agent
    submitted them via the `submit_buy_recommendation` tool. No other text
    or metadata from the agent is returned — hydration into the response
    shape is the caller's responsibility so the agent cannot influence any
    free-text field the buyer sees. Returns [] if the loop ends without
    the agent calling the tool (timed out, refused, or otherwise).
    """
    tools.configure(market_platform_url=config.market_platform_url)

    client = Anthropic(api_key=config.api_key)
    messages: list[dict] = [{"role": "user", "content": user_input}]

    for _ in range(session.max_turns):
        response = client.messages.create(
            model=config.model,
            max_tokens=4096,
            system=session.instructions,
            tools=[tools.SEARCH_TOOL_SCHEMA, tools.SUBMIT_BUY_RECOMMENDATION_SCHEMA],
            messages=messages,
        )

        messages.append({"role": "assistant", "content": response.content})

        if response.stop_reason != "tool_use":
            return []

        submit_block = next(
            (
                b
                for b in response.content
                if b.type == "tool_use" and b.name == "submit_buy_recommendation"
            ),
            None,
        )
        if submit_block is not None:
            listings = (submit_block.input or {}).get("listings") or []
            return [
                {
                    "seller_id": str(item.get("seller_id") or ""),
                    "listing_id": str(item.get("listing_id") or ""),
                }
                for item in listings
                if isinstance(item, dict)
            ]

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

    return []

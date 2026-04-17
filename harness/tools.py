from __future__ import annotations

import requests

# Module-level connection state, set by runner before agent execution.
_market_url: str = ""


def configure(market_platform_url: str) -> None:
    global _market_url
    _market_url = market_platform_url.rstrip("/")


# Anthropic tool schema for `search`. The agent calls into the market platform's
# /api/v1/search endpoint — never OpenSearch directly. The harness is the only
# thing the frontend can reach, and the market platform is the only thing the
# harness can reach for catalog data.
SEARCH_TOOL_SCHEMA = {
    "name": "search",
    "description": (
        "Search the Information Exchange marketplace for data listings using "
        "natural language. Returns matching listings ranked by relevance."
    ),
    "input_schema": {
        "type": "object",
        "properties": {
            "query": {
                "type": "string",
                "description": "Natural language search query.",
            },
            "category": {
                "type": "string",
                "description": "Optional category filter (exact match).",
            },
            "max_results": {
                "type": "integer",
                "description": "Maximum number of results to return.",
                "default": 10,
            },
        },
        "required": ["query"],
    },
}


def search_marketplace(query: str, category: str = "", max_results: int = 10) -> str:
    """Call the market-platform /api/v1/search endpoint and format results."""
    body: dict = {
        "query": query,
        "mode": "hybrid",
        "per_page": max_results,
    }
    if category:
        body["category"] = category

    endpoint = f"{_market_url}/api/v1/search"
    try:
        resp = requests.post(endpoint, json=body, timeout=30)
        resp.raise_for_status()
    except requests.RequestException as e:
        return f"Error reaching market platform at {endpoint}: {e}"

    data = resp.json()
    results = data.get("results") or []
    if not results:
        return "No results found."

    lines = [f"Found {len(results)} results:\n"]
    for i, r in enumerate(results, 1):
        listing_id = r.get("listing_id", "")
        title = r.get("title", "")
        cat = r.get("category", "")
        price_cents = r.get("price_cents", 0)
        score = r.get("score", 0)
        description = r.get("description", "")

        lines.append(f"{i}. **{title}** (ID: {listing_id})")
        lines.append(
            f"   Category: {cat} | Price: ${price_cents / 100:.2f} | Score: {score:.4f}"
        )
        lines.append(f"   {description}\n")

    return "\n".join(lines)


def dispatch(tool_name: str, tool_input: dict) -> str:
    """Dispatch a tool call from the LLM to the matching Python function."""
    if tool_name == "search":
        return search_marketplace(
            query=tool_input.get("query", ""),
            category=tool_input.get("category", ""),
            max_results=int(tool_input.get("max_results", 10)),
        )
    return f"Error: unknown tool '{tool_name}'"

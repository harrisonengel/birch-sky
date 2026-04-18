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


# The agent must end every session by calling this tool with the listings the
# buyer should purchase. The harness intercepts the call, ignores all other
# output from the agent, and deterministically hydrates each {seller_id,
# listing_id} pair from the market platform — so the agent cannot leak free
# text into titles, descriptions, or IDs.
SUBMIT_BUY_RECOMMENDATION_SCHEMA = {
    "name": "submit_buy_recommendation",
    "description": (
        "Finalize the buyer's recommendation. Call this exactly once, at the "
        "end of the session, with the listings the buyer should purchase. "
        "Every id must come verbatim from a prior search result. Do not "
        "invent ids. If no listing is a good fit, call this with an empty "
        "listings array."
    ),
    "input_schema": {
        "type": "object",
        "properties": {
            "listings": {
                "type": "array",
                "description": "Listings to recommend the buyer purchase.",
                "items": {
                    "type": "object",
                    "properties": {
                        "seller_id": {
                            "type": "string",
                            "description": "seller_id from the search result.",
                        },
                        "listing_id": {
                            "type": "string",
                            "description": "listing_id from the search result.",
                        },
                    },
                    "required": ["seller_id", "listing_id"],
                },
            },
        },
        "required": ["listings"],
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
        seller_id = r.get("seller_id", "")
        title = r.get("title", "")
        cat = r.get("category", "")
        price_cents = r.get("price_cents", 0)
        score = r.get("score", 0)
        description = r.get("description", "")

        lines.append(f"{i}. **{title}** (listing_id: {listing_id}, seller_id: {seller_id})")
        lines.append(
            f"   Category: {cat} | Price: ${price_cents / 100:.2f} | Score: {score:.4f}"
        )
        lines.append(f"   {description}\n")

    return "\n".join(lines)


def dispatch(tool_name: str, tool_input: dict) -> str:
    """Dispatch a tool call from the LLM to the matching Python function.

    `submit_buy_recommendation` is intentionally not dispatched here — the
    runner intercepts it to terminate the loop with structured output.
    """
    if tool_name == "search":
        return search_marketplace(
            query=tool_input.get("query", ""),
            category=tool_input.get("category", ""),
            max_results=int(tool_input.get("max_results", 10)),
        )
    return f"Error: unknown tool '{tool_name}'"


class HydrationError(RuntimeError):
    """Raised when the agent's recommended ids don't resolve in the platform."""


def hydrate_buy_listings(recommendations: list[dict]) -> list[dict]:
    """Fetch each recommended listing + seller and project to the response shape.

    Fails loudly (HydrationError) if any id is unknown or if a listing's
    seller_id does not match the submitted seller_id — the whole point of this
    step is that the agent cannot influence any text that reaches the buyer;
    only ids flow through, and everything else is read directly from the
    market platform.
    """
    out: list[dict] = []
    for i, rec in enumerate(recommendations):
        seller_id = str(rec.get("seller_id") or "").strip()
        listing_id = str(rec.get("listing_id") or "").strip()
        if not seller_id or not listing_id:
            raise HydrationError(
                f"recommendation[{i}] missing seller_id or listing_id"
            )

        listing = _get_listing(listing_id)
        if listing is None:
            raise HydrationError(
                f"recommendation[{i}] listing_id {listing_id!r} not found"
            )
        if str(listing.get("seller_id")) != seller_id:
            raise HydrationError(
                f"recommendation[{i}] seller_id {seller_id!r} does not own "
                f"listing_id {listing_id!r}"
            )

        seller = _get_seller(seller_id)
        if seller is None:
            raise HydrationError(
                f"recommendation[{i}] seller_id {seller_id!r} not found"
            )

        out.append(
            {
                "id": listing["id"],
                "price": int(listing.get("price_cents") or 0),
                "listing_description": listing.get("description") or "",
                "seller": seller.get("name") or "",
            }
        )
    return out


def _get_listing(listing_id: str) -> dict | None:
    endpoint = f"{_market_url}/api/v1/listings/{listing_id}"
    resp = requests.get(endpoint, timeout=30)
    if resp.status_code == 404:
        return None
    resp.raise_for_status()
    return resp.json()


def _get_seller(seller_id: str) -> dict | None:
    endpoint = f"{_market_url}/api/v1/sellers/{seller_id}"
    resp = requests.get(endpoint, timeout=30)
    if resp.status_code == 404:
        return None
    resp.raise_for_status()
    return resp.json()

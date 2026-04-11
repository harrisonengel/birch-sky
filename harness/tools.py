from __future__ import annotations

import requests

# Module-level connection state, set by runner before agent execution.
_os_url: str = ""
_os_index: str = ""
_os_auth: tuple[str, str] | None = None


def configure(url: str, index: str, user: str | None, password: str | None) -> None:
    global _os_url, _os_index, _os_auth
    _os_url = url.rstrip("/")
    _os_index = index
    _os_auth = (user, password) if user else None


# Anthropic tool schema for search_opensearch. Mirrors the Go TextSearch
# shape in src/market-platform/internal/search/opensearch.go and the
# MCP search_marketplace tool.
SEARCH_TOOL_SCHEMA = {
    "name": "search_opensearch",
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


def search_opensearch(query: str, category: str = "", max_results: int = 10) -> str:
    """Query OpenSearch and format results for the agent."""
    filters: list[dict] = [{"term": {"status": "active"}}]
    if category:
        filters.append({"term": {"category": category}})

    search_body = {
        "size": max_results,
        "query": {
            "bool": {
                "must": [
                    {
                        "combined_fields": {
                            "query": query,
                            "fields": [
                                "title^3",
                                "description^2",
                                "tags^2",
                                "content_text",
                            ],
                            "operator": "or",
                        }
                    }
                ],
                "filter": filters,
            }
        },
    }

    endpoint = f"{_os_url}/{_os_index}/_search"
    try:
        resp = requests.post(
            endpoint,
            json=search_body,
            auth=_os_auth,
            timeout=30,
        )
        resp.raise_for_status()
    except requests.RequestException as e:
        return f"Error reaching OpenSearch at {endpoint}: {e}"

    data = resp.json()
    hits = data.get("hits", {}).get("hits", [])
    if not hits:
        return "No results found."

    lines = [f"Found {len(hits)} results:\n"]
    for i, hit in enumerate(hits, 1):
        src = hit.get("_source", {})
        score = hit.get("_score", 0)
        listing_id = src.get("listing_id", "")
        title = src.get("title", "")
        cat = src.get("category", "")
        price_cents = src.get("price_cents", 0)
        description = src.get("description", "")

        lines.append(f"{i}. **{title}** (ID: {listing_id})")
        lines.append(
            f"   Category: {cat} | Price: ${price_cents / 100:.2f} | Score: {score:.4f}"
        )
        lines.append(f"   {description}\n")

    return "\n".join(lines)


def dispatch(tool_name: str, tool_input: dict) -> str:
    """Dispatch a tool call from the LLM to the matching Python function."""
    if tool_name == "search_opensearch":
        return search_opensearch(
            query=tool_input.get("query", ""),
            category=tool_input.get("category", ""),
            max_results=int(tool_input.get("max_results", 10)),
        )
    return f"Error: unknown tool '{tool_name}'"

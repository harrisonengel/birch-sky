# mcp/ — Design

MCP (Model Context Protocol) server providing tools for buyer agents.

## Transport

SSE (Server-Sent Events) on port 8081, using `github.com/mark3labs/mcp-go`.

## Tools

| Tool | Purpose |
|------|---------|
| `search_marketplace` | Natural language search with optional category/price filters |
| `get_listing` | Full public metadata for a listing by ID |
| `analyze_data` | AI-powered data analysis without revealing raw data |

## `analyze_data` — Arrow's Paradox Resolver

The key tool that enables the marketplace. A buyer agent asks questions about a dataset; the service loads the raw data from MinIO, sends it to Claude API with strict instructions to:
- Answer questions with summaries, trends, and patterns
- Never include raw data values or specific records
- Prevent prompt injection from crafted datasets

This allows buyers to evaluate data quality and relevance before purchasing, without "stealing" the information.

## DataAnalyzer Interface

- `ClaudeAnalyzer`: production, uses `anthropic-sdk-go` with Claude 3.5 Sonnet
- `StubAnalyzer`: testing, returns placeholder responses

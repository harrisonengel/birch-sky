# Agent Harness Spec

## Overview

A standalone, config-driven Python agent runner that lets a buyer's agent
query the IE marketplace (OpenSearch) without running the full server stack.
The Go CLI is a thin cobra wrapper; all logic lives in Python. The harness
uses the Anthropic SDK directly with a small custom tool loop — no agent
framework or litellm — to keep the dependency surface minimal.

## Architecture

```
Go CLI (cobra)  →  subprocess  →  Python harness (anthropic SDK + custom loop)
                                      │
                                      ▼
                                  OpenSearch (raw HTTP via requests)
```

### Components

| Component | File | Purpose |
|-----------|------|---------|
| Config loader | `harness/config.py` | Load YAML, resolve env vars |
| OpenSearch tool | `harness/tools.py` | `search_opensearch` + Anthropic tool schema + `dispatch` |
| Session state | `harness/session.py` | Build system prompt from config |
| Runner | `harness/runner.py` | Custom Anthropic tool loop (messages.create + tool_result) |
| Entry point | `harness/__main__.py` | argparse CLI |
| Go CLI | `src/market-platform/cmd/agent/` | cobra wrapper calling `python -m harness` |

### Config schema (`config.example.yaml`)

```yaml
model:
  name: "claude-sonnet-4-5"                 # Anthropic model ID
  api_key_env: "ANTHROPIC_API_KEY"          # env var name holding the key
opensearch:
  url: "http://localhost:9200"
  index: "listings"                         # matches mapping.go IndexName
  # user: ""   # optional basic auth
  # pass: ""
session:
  mode: "context"
  max_turns: 20
  starting_context:
    background: "You work for a hedge fund..."
    goal: "Find datasets related to satellite imagery..."
    constraints: "Budget under $500 per dataset."
```

### Search query compatibility

The Python tool builds the same `combined_fields` query as the Go
`TextSearch` method (`opensearch.go:132-148`):

- Fields: `title^3`, `description^2`, `tags^2`, `content_text`
- Operator: `or`
- Filters: `status:active` always, optional `category` term

Output formatting matches `server.go:96-99`.

## Dependencies

- `anthropic>=0.87.0` — official Anthropic Python SDK (pinned above CVE-2026-34450/34452)
- `pyyaml>=6.0.2` — config parsing (always use `yaml.safe_load`)
- `requests>=2.33.0` — raw HTTP to OpenSearch (no `opensearch-py`)
- `spf13/cobra` — Go CLI framework (project standard)

`litellm` and `openai-agents` were considered but dropped after the
LiteLLM supply chain compromise (March 2026, CVE coverage of v1.82.7/8).
Writing a ~30-line tool loop against the Anthropic SDK directly avoids
both dependencies entirely. See
`docs/security/dependency-analysis/spf13-cobra.md` for the Go deps review.

## Future work

- Resumable sessions (session file save/restore)
- Additional tools: `get_listing`, `analyze_data`
- Vector search support
- Additional model providers (OpenAI, Bedrock) via thin per-provider loops

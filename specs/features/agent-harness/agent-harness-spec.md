# Agent Harness Spec

## Overview

A standalone, config-driven Python agent runner that lets a buyer's agent
query the IE marketplace (OpenSearch) without running the full server stack.
The harness uses the Anthropic SDK directly with a small custom tool loop —
no agent framework or litellm — to keep the dependency surface minimal.

The harness is a Python script today. The core loop (`harness.runner.execute`)
takes in-memory `HarnessConfig` and `Session` objects, so it is ready to be
wrapped in an HTTP endpoint as a next step without further refactoring.

## Architecture

```
python -m harness  →  anthropic SDK tool loop  →  OpenSearch (raw HTTP)
```

### Components

| Component | File | Purpose |
|-----------|------|---------|
| Config loader | `harness/config.py` | Load infrastructure YAML (model, opensearch), resolve env vars |
| Session loader | `harness/session.py` | Load per-call session YAML (starting_context, max_turns) and build system prompt |
| OpenSearch tool | `harness/tools.py` | `search_opensearch` + Anthropic tool schema + `dispatch` |
| Runner | `harness/runner.py` | `execute(config, session, input) -> str` tool loop + `run(...)` CLI wrapper |
| Entry point | `harness/__main__.py` | argparse CLI — takes `-c` config, `-s` session, `-i` input |

### Config vs. session split

The harness separates **infrastructure config** from **per-invocation session
state**:

- **Config** (`-c config.yaml`) is stable, reusable across calls, and lives on
  disk alongside deployment. It contains the model endpoint and search
  backend — nothing about *what* the agent is trying to do.
- **Session** (`-s session.yaml`) is the call itself. Each invocation of the
  harness provides its own session describing the agent's starting context
  (background, goal, constraints) and turn budget. A caller running many
  different buyer queries against the same config will produce many
  different session payloads.
- **Input** (`-i "..."`) is the initial user message seeding the first turn.

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
```

### Session schema (`session.example.yaml`)

```yaml
max_turns: 20
starting_context:
  background: "You work for a hedge fund..."
  goal: "Find datasets related to satellite imagery..."
  constraints: "Budget under $500 per dataset."
```

### Invocation

```bash
python -m harness -c config.yaml -s session.yaml -i "find satellite data"
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

`litellm` and `openai-agents` were considered but dropped after the
LiteLLM supply chain compromise (March 2026, CVE coverage of v1.82.7/8).
Writing a ~30-line tool loop against the Anthropic SDK directly avoids
both dependencies entirely. See
`docs/security/dependency-analysis/python-harness-deps.md` for details.

## Future work

- **HTTP API.** Wrap `harness.runner.execute` in a FastAPI (or Flask)
  endpoint exposing `POST /run` with a `{starting_context, user_input,
  max_turns}` body. The runner already takes in-memory objects, so this
  is a small additive change (~40 lines + one dependency). A dedicated
  CLI for interacting with the API should be added at that point as a
  proper HTTP client — not a subprocess wrapper.
- Resumable sessions: extend the session file to carry prior transcript and
  support `--resume` across multi-turn workflows
- Additional tools: `get_listing`, `analyze_data`
- Vector search support
- Additional model providers (OpenAI, Bedrock) via thin per-provider loops

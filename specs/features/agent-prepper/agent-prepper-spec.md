# Agent Prepper Spec

## Motivation

Once a buyer's agent enters the IE walled marketplace, no free-form information
can come back out — that is the core "forgetful buyer" guarantee that resolves
Arrow's Information Paradox for the seller. Every fact the in-marketplace agent
needs about the buyer's intent — what they want, what counts as a good dataset,
whether to compute an answer end-to-end or just shop for inputs — must be
fixed *before* the `enter` API is called.

Today the web app sends the buyer's raw query straight to `POST /api/v1/enter`
(market-platform) or `POST /agent/run` (harness). Real users under-specify.
Deep-research products solve this with a brief clarification loop in front of
the main run. `agent-prepper` is that loop for IE.

## Scope

A standalone Python microservice that holds a short multi-turn conversation
with the buyer in the **free-form web app** (outside the walled system) and
emits a structured **Briefing** that the harness consumes as
`starting_context`.

The prepper is invoked by the web app only. The market-platform `enter` API
is unchanged.

## Required output (`Briefing`)

The clarification loop terminates only when all three of the following are
known with reasonable confidence:

| Field              | Type                                          | Purpose |
|--------------------|-----------------------------------------------|---------|
| `goal_summary`     | string                                        | What the buyer ultimately wants to achieve. Carried into the agent's system prompt as `goal`. |
| `selection_criteria` | string[]                                    | Concrete requirements any candidate seller dataset must satisfy (domain, freshness, geography, schema, resolution, license, etc.). Carried into the agent's system prompt as part of `constraints`. |
| `analysis_mode`    | `"compute_to_end"` \| `"evaluate_then_decide"` | Whether the in-marketplace agent should attempt to compute the full analysis before deciding what to buy, or just evaluate dataset fit and decide. Carried in as `constraints`. |
| `background`       | string (optional)                             | Buyer org / context. |
| `constraints`      | string (optional)                             | Budget, timeline, legal/jurisdiction limits. |

`analysis_mode` semantics:
- `compute_to_end` — the agent attempts the analysis on candidate data inside
  the walled system; final purchase decision is informed by which dataset
  actually answers the question. More token cost, better answers.
- `evaluate_then_decide` — the agent inspects schema/samples/metadata, picks
  the dataset, and stops. Cheaper, faster, suitable when the buyer just wants
  the data not the answer.

## Architecture

```
web app  →  POST /api/prepper/start       →  agent-prepper
         →  POST /api/prepper/respond  ←─┘  (Anthropic tool loop)
         (loop until status == "ready")
         →  Briefing
         →  POST /agent/run with starting_context = Briefing
```

### Components

The service ("agent-prepper") lives in the Python package `agent_prepper/`
(underscore so it's importable). The hyphenated name is used in the
service / docker / URL surface only.

| Component | File | Purpose |
|-----------|------|---------|
| Config    | `agent_prepper/config.py`   | Env-var config (model, api_key, max_turns) — mirrors `harness/config.py` |
| Prompts   | `agent_prepper/prompts.py`  | System prompt + `finalize_context` tool schema |
| Session   | `agent_prepper/session.py`  | `PrepperSession` dataclass + in-memory store |
| Runner    | `agent_prepper/runner.py`   | `execute_turn(session, user_msg) -> TurnResult` Anthropic tool loop |
| API       | `agent_prepper/api.py`      | FastAPI: `/api/prepper/{start,respond,session/{id}}` + `/health` |
| Entry     | `agent_prepper/__main__.py` | `python -m agent_prepper` for local dev |

### Tool loop

The model is given exactly one tool: `finalize_context`. Each call to
`POST /api/prepper/respond`:

1. Appends the user's `answer` to the session transcript.
2. Calls `client.messages.create` with `tools=[FINALIZE_CONTEXT_TOOL]`.
3. If the model returns a `tool_use` block calling `finalize_context`,
   validate the arguments, store the resulting `Briefing` on the session,
   set `status="ready"`, and return it.
4. Otherwise, return the model's text as the next clarifying `question`
   and keep the session at `status="asking"`.

A hard turn cap (`PREPPER_MAX_TURNS`, default 6) forces a finalize on the
last turn — the system prompt instructs the model to make the best Briefing
it can from whatever has been gathered rather than refusing.

### Session storage (MVP)

In-process `dict[str, PrepperSession]` keyed by UUID. Not persistent across
restarts. This is acceptable for the MVP since:

- A web-app session that loses its prepper session can simply restart the
  conversation; nothing has been spent yet.
- Persistence + multi-replica scaling is a follow-up spec, not needed to
  validate the UX.

## HTTP API

All routes are namespaced under `/api/prepper/`.

### `POST /api/prepper/start`
Request:
```json
{ "buyer_id": "buyer-abc123", "initial_query": "I want to understand EV demand" }
```
Response:
```json
{
  "session_id": "uuid",
  "status": "asking",
  "question": "What region should the EV demand data cover?",
  "turn": 1
}
```
Or, if the model finalizes immediately (rare):
```json
{
  "session_id": "uuid",
  "status": "ready",
  "briefing": { ...Briefing... },
  "turn": 1
}
```

### `POST /api/prepper/respond`
Request:
```json
{ "session_id": "uuid", "answer": "US, last 12 months." }
```
Response: same shape as `/start`, advancing `turn` by 1.
Returns `404` if `session_id` unknown, `409` if session is already `ready`.

### `GET /api/prepper/session/{id}`
Returns the full session state for debugging and the CLI:
```json
{
  "session_id": "uuid",
  "buyer_id": "buyer-abc123",
  "status": "asking" | "ready",
  "turn": 3,
  "transcript": [{"role": "user"|"assistant", "content": "..."}],
  "briefing": null | { ... }
}
```

### `GET /health`
`{"status": "ok"}`

## Frontend integration (`src/demo-flow.js`)

A small state machine wraps the existing flow:

- `mode = "prepping"` — first user message kicks off `startPrepper(query)`.
  Subsequent user messages route to `respondPrepper(sessionId, answer)`.
  Each `question` in the response renders as an agent chat bubble.
- On `status === "ready"` — transition to `mode = "running"` and call the
  existing `runFlow` (or the harness `runAgent`) using the Briefing as
  `starting_context`.
- Fallback — if the prepper service is unreachable on the first call, fall
  back to the current direct `enterMarketplace(query)` path. Matches the
  existing `enterBackend` fallback pattern. This also keeps Playwright
  tests deterministic when prepper is not running.
- Demo toggle — when `#demo-mode` is forced to `results` or `no-results`,
  skip prepper entirely. Only `auto` engages the clarification loop.

## CLI (Go + cobra)

Per CLAUDE.md, every API ships with a CLI. `src/market-platform/cmd/prepper-cli/main.go`:

```
prepper-cli start    --query "..."   --buyer-id X
prepper-cli respond  --session-id S  --answer "..."
prepper-cli chat     --query "..."   [--buyer-id X]    # interactive stdin loop
prepper-cli session  --session-id S                    # dump session JSON
```

`--prepper-url` flag, defaults to `$PREPPER_URL` or `http://localhost:8002`.

## Docker

Add `agent-prepper` service to `src/market-platform/docker-compose.yml`:

```yaml
agent-prepper:
  build:
    context: ../../agent_prepper
    dockerfile: Dockerfile
  ports:
    - "8002:8002"
  environment:
    MODEL_NAME: claude-sonnet-4-5
    ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}
    PREPPER_MAX_TURNS: "6"
```

Vite dev proxy adds `/api/prepper -> http://localhost:8002`.

## Dependencies

- `anthropic>=0.87.0` (same pin as harness, above CVE-2026-34450/34452)
- `fastapi>=0.115.0`
- `uvicorn[standard]>=0.34.0`
- `pydantic>=2.0` (transitive via FastAPI)

No `litellm` for the same supply-chain reason as the harness.

## Verification

1. `python -m agent_prepper` locally → `curl /health` returns ok.
2. `prepper-cli chat --query "I want to understand EV demand"` runs an
   interactive loop that terminates within `PREPPER_MAX_TURNS` and prints
   a Briefing JSON containing `goal_summary`, `selection_criteria`, and
   `analysis_mode`.
3. `docker compose up agent-prepper` brings the service up; health probe
   passes.
4. Frontend: `npm run dev`, type a vague query — clarifying questions
   render in chat, then the existing results flow runs once `status ==
   ready`. Network tab shows the Briefing carried into `/agent/run`.
5. Playwright: `e2e/flows/prepper-clarification.spec.js` covers the
   fallback path (prepper unreachable → direct enter still works) and,
   under a stub mode, the multi-turn clarification UX.

## Future work

- Persistent session storage (Postgres) — required before multi-replica.
- Reuse of buyer profile across sessions (carry forward `background`).
- Streaming responses so the prepper question can render token-by-token.
- Authn via Cognito JWT (`buyer_id` derived from claims, not client-supplied).
- Telemetry: log Briefings + downstream agent outcomes to feed the trust
  engine's scoring of how good the prepper is at producing useful intent.

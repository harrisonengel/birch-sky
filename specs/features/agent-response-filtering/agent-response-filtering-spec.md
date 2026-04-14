# Agent Response Filtering: Preventing Data Leakage to the Frontend

## Problem

The Information Exchange's core security invariant is that buyer agents
"forget" any information they didn't pay for when they leave the market.
Today this invariant has no enforcement in the harness-to-frontend data path.
An audit of the current MVP identified three issues:

1. **Raw agent text returned to frontend.** `harness/api.py` returns the
   agent's complete free-form text response via `RunResponse(response=result)`.
   The agent can reference seller data, relevance scores, internal reasoning,
   and content from unpurchased listings. Nothing prevents this text from
   reaching the buyer's browser. `runAgent()` is defined in `src/api-client.js`
   but not yet called — the leak is dormant but one import away from active.

2. **No structured output contract.** The harness returns `str`. There is no
   schema defining what the frontend is allowed to see. When the agent is
   wired in, the frontend will receive whatever Claude decides to say.

3. **innerHTML XSS in result cards.** `src/chat.js:50-58` renders
   marketplace search results using `innerHTML` with unescaped seller-supplied
   fields (`r.title`, `r.seller`, `r.description`). A malicious seller could
   inject HTML/JS through listing metadata. This is not an agent data leak
   but was discovered during the same audit and shares the same fix boundary
   (the rendering path).

## Design Principles

- **The frontend receives structured recommendations, never free-form agent
  text.** The agent's prose stays inside the harness boundary.
- **The harness is the trust boundary.** Filtering happens server-side in
  Python, not client-side in JavaScript. The frontend cannot be trusted to
  discard data it has already received.
- **Listing metadata shown to the buyer must come from the marketplace API,
  not from the agent.** The agent provides listing IDs and a purchase
  recommendation; the frontend fetches display data from the market platform.
- **Defence in depth.** Even if structured output parsing fails, the fallback
  must not leak raw text.

## Changes

### 1. Define a structured response schema (harness)

Replace the free-form `RunResponse(response: str)` with a structured model
that separates what the buyer can see from what stays inside the harness.

**New file: `harness/response.py`**

```python
from __future__ import annotations
from pydantic import BaseModel


class ListingRecommendation(BaseModel):
    """A single listing the agent recommends the buyer consider."""
    listing_id: str
    relevance: str          # "high" | "medium" | "low"
    reason: str             # one-sentence, agent-generated rationale


class AgentResponse(BaseModel):
    """Structured output returned to the frontend.

    Contains only information the buyer is allowed to see:
    - A summary of what was found (no seller data details)
    - Listing IDs with relevance + rationale (frontend fetches
      display metadata from the marketplace API by ID)
    - Whether the agent recommends posting a buy request
    """
    summary: str                              # e.g. "Found 3 relevant datasets"
    recommendations: list[ListingRecommendation]
    suggest_buy_request: bool                 # true if no good matches
    buy_request_draft: str | None = None      # prefilled query for the form
```

### 2. Add a response extractor (harness)

The agent's raw text must be parsed into the structured schema above. This
happens inside the harness, before the HTTP response is built.

**New file: `harness/extractor.py`**

Approach: use a second, cheap LLM call (same model, low `max_tokens`) with
a JSON-mode prompt that takes the agent's raw output and returns a
`AgentResponse`-shaped JSON object. This is a "distillation" call — it sees
the agent's full output but only emits the structured fields.

```python
def extract(raw_agent_text: str, client: Anthropic, model: str) -> AgentResponse:
    """Parse the agent's raw output into a safe structured response.

    Uses a constrained LLM call to extract structured fields.
    Falls back to a safe empty response on any parse failure.
    """
```

**Fallback behaviour:** if extraction fails (malformed JSON, timeout, etc.),
return a safe default:

```python
AgentResponse(
    summary="I searched the Exchange but couldn't extract structured results. Please try again.",
    recommendations=[],
    suggest_buy_request=False,
)
```

This guarantees the raw agent text never reaches the frontend, even on
error paths.

**Why a second LLM call instead of regex/heuristics?** The agent's output
is free-form prose. Regex extraction is brittle and will break as the system
prompt evolves. A constrained extraction call is robust and self-maintaining.
The cost is low — small input, ~200 output tokens, same model already loaded.

### 3. Update the harness API endpoint

**File: `harness/api.py`**

- Replace `RunResponse(response: str)` with the new `AgentResponse` model.
- After `execute()` returns the raw string, pass it through `extract()`.
- Return the `AgentResponse` as JSON.

```python
@app.post("/api/run", response_model=AgentResponse)
def run_agent(req: RunRequest) -> AgentResponse:
    config = _load_config()
    session = from_context(req.starting_context, max_turns=req.max_turns)
    raw = execute(config, session, req.user_input)
    return extract(raw, Anthropic(api_key=config.api_key), config.model)
```

The old `RunResponse` model is deleted entirely. There is no `response: str`
field in the new schema.

### 4. Update the frontend API client

**File: `src/api-client.js`**

Update `runAgent()` return type documentation and field access to match the
new structured response:

```javascript
/**
 * Run the buyer agent via the harness service.
 * @param {string} userInput
 * @param {object} context - {background, goal, constraints}
 * @returns {Promise<{
 *   summary: string,
 *   recommendations: Array<{listing_id: string, relevance: string, reason: string}>,
 *   suggest_buy_request: boolean,
 *   buy_request_draft: string|null
 * }>}
 */
export async function runAgent(userInput, context) { ... }
```

### 5. Fix innerHTML XSS in result cards

**File: `src/chat.js`**

Replace `innerHTML` template string construction in `addResults()` with
explicit DOM element creation using `textContent` for all seller-supplied
strings:

```javascript
function addResults(results) {
  const container = document.createElement('div');
  // ...
  results.forEach((r, i) => {
    const card = document.createElement('div');
    card.className = 'result-card';

    const title = document.createElement('div');
    title.className = 'result-title';
    title.textContent = r.title;
    card.appendChild(title);

    const seller = document.createElement('div');
    seller.className = 'result-seller';
    seller.textContent = r.seller;
    card.appendChild(seller);

    const desc = document.createElement('div');
    desc.className = 'result-desc';
    desc.textContent = r.description;
    card.appendChild(desc);

    // ... footer with trust bar (static structure, no user data in HTML) ...
  });
}
```

Also apply the same treatment to `addBuyRequestForm()` — the prefill values
come from user input and should use `value` property assignment, not
template interpolation:

```javascript
titleInput.value = prefill.title;    // not innerHTML with ${prefill.title}
descTextarea.value = prefill.description;
```

### 6. Update the demo flow for agent integration

**File: `src/demo-flow.js`**

When the agent harness is connected, `runFlow()` should:

1. Call `runAgent(query, context)` instead of `searchMarketplace(query)`.
2. Display `response.summary` as a chat message.
3. If `response.recommendations` is non-empty, fetch listing metadata from
   the marketplace API by ID and render result cards.
4. If `response.suggest_buy_request` is true, show the buy request form
   pre-filled with `response.buy_request_draft`.

This is a future wiring step. The current `searchMarketplace` path remains
as the fallback when the harness is not available.

## Implementation order

| Step | File(s) | Depends on | Agent |
|------|---------|------------|-------|
| 1 | `harness/response.py` | — | `/programmer` |
| 2 | `harness/extractor.py` | Step 1 | `/programmer` |
| 3 | `harness/api.py` | Steps 1, 2 | `/programmer` |
| 4 | `src/api-client.js` | Step 3 | `/programmer` |
| 5 | `src/chat.js` | — | `/programmer` |
| 6 | `src/demo-flow.js` | Steps 4, 5 | `/programmer` |
| 7 | Tests | Steps 1-6 | `/quality-assurance` |

Steps 1-3 (harness changes) and Step 5 (XSS fix) are independent and can
be done in parallel.

## Testing

### Harness unit tests

- **Extractor happy path:** given a mock agent response containing listing
  IDs and reasoning, `extract()` returns a valid `AgentResponse` with the
  correct listing IDs and no raw prose.
- **Extractor fallback:** given malformed agent output, `extract()` returns
  the safe default response with empty recommendations.
- **No raw text leak:** assert that `AgentResponse` has no field that could
  carry arbitrary agent prose. The `summary` field must be limited in length
  (enforce with `Field(max_length=300)`) and the `reason` field per
  recommendation likewise (`Field(max_length=200)`).
- **API integration:** `POST /api/run` returns a response matching the
  `AgentResponse` schema. No `response` key exists in the JSON.

### Frontend unit tests (Vitest)

- **XSS prevention:** render `addResults()` with a result whose `title`
  contains `<script>alert(1)</script>`. Assert the script tag appears as
  escaped text, not as an executable element.
- **Structured response handling:** mock `runAgent()` returning the new
  schema. Verify the chat displays `summary` text and listing cards, not
  raw agent prose.

### E2E tests (Playwright)

- **Agent flow with structured response:** mock the `/agent/run` endpoint
  to return a valid `AgentResponse`. Verify the UI shows result cards
  matching the recommended listing IDs.
- **Agent flow with empty recommendations:** mock a response with
  `suggest_buy_request: true`. Verify the buy request form appears.

## Non-goals

- **Prompt engineering the agent to not leak.** System-prompt constraints
  are not a security boundary. The agent will always be able to produce
  free-form text; the filtering must happen structurally.
- **Client-side filtering.** Any data that reaches the browser is
  compromised. Filtering must be server-side.
- **Streaming agent output.** Streaming is deferred. The extraction step
  requires the full agent response before it can produce structured output.
  If streaming is added later, it must stream the *extracted* structured
  response, not the raw agent text.

## Rationale

### Why not use tool_use / structured output on the primary agent call?

Anthropic's API supports `tool_use` for structured output, and we could
constrain the agent to only respond via a `respond_to_buyer` tool. This
would avoid the second extraction call. However:

- It changes the agent's behavior: tool-use-only responses alter how the
  model reasons and may reduce analysis quality.
- The agent's system prompt is controlled by the `Session`, which comes
  from the buyer's `starting_context`. If the buyer can influence the
  system prompt, they might instruct the agent to leak via the tool's
  free-text fields.
- A separate extraction call creates a clean architectural boundary:
  the agent runs unconstrained for maximum analysis quality, then a
  second pass distills the output into a safe shape. The extractor's
  system prompt is hardcoded and not caller-influenced.

If future benchmarking shows the extraction call adds unacceptable latency,
this decision can be revisited — but the structured output schema and the
"no raw text to frontend" invariant must be preserved regardless of
implementation.

### Why enforce max_length on summary and reason?

Without length limits, the `summary` or `reason` fields become vectors for
smuggling large amounts of agent prose to the frontend. A 300-character
summary and 200-character reason are sufficient for useful buyer-facing
content while making it impractical to embed full seller data descriptions.

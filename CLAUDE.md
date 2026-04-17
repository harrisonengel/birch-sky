# Project Context: The Information Exchange

## Executive Summary

IE is a data marketplace that enables the commodification of information. Formerly impractical because data could not be adequately analyzed before purchase (Arrow's Information Paradox), new trusted AI infrastructure at IE allows buyers to request, find, trust, and use purchaseable information from sellers around the world before making the purchase and without "stealing" the information. This is done through *buyers agents* who enter (or more specifically are cloned into) our walled system and can see and run their analysis with full access to seller data. Their agent can tell them what information to buy, and the exchange brokers the final transaction. If none of the sellers across the market meet the needs of the buyer, they pay nothing, and the agent never returns to reveal the secrets it learned.

The exchange hosts the information for sale, provides the buyer agent platform, and makes money as a broker between the two.

The product is three layers.
The base layer is a raw information market. Sellers post information for sale at a price, buyers agents have tools to search and use this information. Buyers agents can also post requests for information, and sellers can compete to fill those information requests.
The second layer is a trust engine that verifies sellers, cross-references claims, and builds longitudinal accuracy scores from transaction outcomes. This layer is a moat that grows stronger as it learns about the sellers in the market. It provides confidence levels to buyers agents for the information they see.
The third is the buyers agent platform, the AI-powered intelligence layer that hosts buyers agents and allows them to find information to purchase.

This marketplace will form an indispensable backbone of the agent economy, and represents the first of its kind true information marketplace.

> Key insight: prior data marketplaces failed because they were supply-side catalogs of high level descriptions of the information. This was necessary because of Arrow's paradox. However, LLM based AI technology enables a new kind of dense information marketplace using "forgetful buyers agents", AI agents that have the knowledge and goals of their organization but "forget" any information they didn't pay for when they leave the market.

## Coding Behavior

Behavioral guidelines to reduce common LLM coding mistakes.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

### 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

### 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

## Workflow Rules

- All plans must become specs checked in to `specs/` before implementation begins.
- Name spec files descriptively (e.g. `mvp-auth-design.md`), not `README.md`.
- Specs live under `specs/features/<feature-name>/` or `specs/<area>/` as appropriate.

## API & CLI Tooling Rules

- **Every new API or service must ship with a CLI tool** (or extend an
  existing CLI tool) that lets a human or Claude exercise the API end
  to end. The CLI is what we will use to populate real demo data, so
  shipping an API without one is incomplete work.
- **CLI tools are written in Go using [spf13/cobra](https://github.com/spf13/cobra).**
  This is the CLI framework we standardize on across the project.

## Branch & PR Rules

- **Never commit directly to master.** All work happens on a feature branch.
- At the start of a session, create or continue work on a descriptive branch (e.g. `add-devops-persona`, `fix-auth-flow`).
- Raise a PR early and update it as you go — commits should be incremental, not one giant squash at the end.
- Every Claude session must end with all changes committed, pushed, and associated with a PR. Do not leave uncommitted work.
- PRs require code review before merging. Do not merge your own PRs without review.

## Testing

The project uses Vitest (unit) + Playwright (E2E). Both are required CI gates.

### Before submitting any PR

The JS test suites (`npm test`, `npm run test:e2e`) are only required when your PR changes the **application stack** — i.e. anything under `src/`, `index.html`, `vite.config.js`, `vitest.config.js`, `playwright.config.js`, `e2e/`, or `package.json`/`package-lock.json`. PRs that only touch peripheral code (e.g. `orchestrator/`, `scripts/`, `docs/`, `specs/`, `.claude/`, CLI tools under `arch/` or `fin/`) do not need to run the JS suites.

When your PR touches the application stack:

1. Run `npm test` — resolve all failures before proceeding.
2. Run `npm run test:e2e` — resolve all failures before proceeding.
3. If you introduced new UI functionality, add a corresponding Playwright test in `e2e/flows/`.
4. Do not submit a PR with known test failures under any circumstance.

### Test authoring guidelines

- Vitest: co-locate test files next to the module as `<name>.test.js` under `src/`.
- Playwright: place E2E tests in `e2e/flows/` named after the user journey.
- Test behavior, not implementation — avoid asserting on class names or internal state.
- Prefer `getByRole` and `getByText` selectors in Playwright over CSS selectors. Use CSS selectors only when there is no accessible alternative (e.g. the demo-mode `<select>`).
- Make E2E tests deterministic. The demo flow has a `#demo-mode` toggle (`results` / `no-results`) — set it explicitly rather than relying on the auto-alternate behavior.

### When tests fail

- Read the full error output before attempting a fix.
- Do not delete or skip tests to make the suite pass.
- If a test is genuinely wrong (testing the wrong thing), explain why in the PR description before modifying it.
- Every bug found in production must get a regression test (Playwright if it's a user-visible flow, Vitest if it's pure logic) before the fix is merged.

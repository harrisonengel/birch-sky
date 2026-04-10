# MVP Trust Strategy: Curated Onboarding + Data Collection + Manual Refutation

## Decision

The trust engine (Layer 2 of IE) is **deferred post-MVP**. Automated trust scores, longitudinal accuracy scoring, and seller cross-referencing are not being built now.

### What replaces it in the MVP

**Seller trust**: Established through manual, curated onboarding. Every seller in the MVP is personally vetted and onboarded by the operator. There is no self-serve seller registration. This sidesteps the trust problem entirely at small scale — the operator is the trust engine.

**Purchase data collection**: Even though we are not computing trust scores yet, every transaction must capture as much metadata as possible so the trust engine has rich signal to learn from when it is built. See [Data to Collect](#data-to-collect) below.

**Refutation / refund flow**: Entirely manual. Buyers who believe purchased data did not meet the description contact the operator via an email address displayed on the website. The operator reviews and processes any refund manually via Stripe. There is no API, no automation, and no in-product flow for refutations in the MVP.

---

## Rationale

Automated trust scoring requires a critical mass of transaction outcomes before it produces useful signal. Building the scoring engine before that data exists would be premature. Curated onboarding costs nothing at MVP scale (O(10) sellers) and produces the same outcome — buyers can trust the data — without the engineering overhead.

The risk accepted is that this does not scale past ~O(100) sellers. That is fine: scaling seller trust is a post-MVP problem, and the transaction data collected now seeds the engine that solves it.

---

## Data to Collect

Every completed purchase must record the following. This is the raw material for future trust scoring.

### Transaction record (already in schema)
- `buyer_id`, `listing_id`, `amount_cents`, `stripe_payment_id`
- `status`, `created_at`, `completed_at`

### To add to the transactions table
| Field | Type | Purpose |
|---|---|---|
| `buyer_agent_query` | `TEXT` | The original query the buyer agent was issued — ground truth for relevance scoring later |
| `agent_analysis_summary` | `TEXT` | The `analyze_data` response the agent received — captures what the agent concluded about the data |

---

## Refutation Flow (MVP)

Buyers contact the operator by email. The operator's contact email is displayed on the website (e.g. on the purchase confirmation screen and in a support/contact page).

There is no `/refute` endpoint, no in-product dispute form, and no automated notification. The operator handles everything out-of-band.

**Post-MVP**: When volume warrants it, this becomes a structured in-product flow with API endpoints, automated operator notifications, and tracked refutation outcomes that feed the trust engine.

---

## Future Trust Engine Hook Points

When the trust engine is built, it will consume:
- The `buyer_agent_query` + `agent_analysis_summary` pair to assess whether data answered the stated need
- Refutation outcomes (once tracked) as ground-truth accuracy labels per seller
- Transaction volume and refutation rate per seller over time

The schema additions above are designed so the trust engine can read from `transactions` directly without a migration.

---

## Impact on Existing Specs

- `docs/specs/market-platform/market-platform-service.md`: The note "trust scores are out of scope" is correct but understated. This spec supersedes it for trust strategy. The transactions schema in that spec should be extended with the fields above.
- `CLAUDE.md` executive summary describes the trust engine as Layer 2. That description remains accurate — it is still the plan, just deferred.

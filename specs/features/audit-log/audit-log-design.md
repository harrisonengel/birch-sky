# Audit Log -- Architecture and Implementation Spec

## Context

The Information Exchange is a financial data marketplace where AI agents access seller data inside a sandbox, and real money changes hands via Stripe. Three forces demand a comprehensive audit trail from day one:

1. **Dispute resolution.** The MVP refutation flow is manual (buyer emails operator). The operator needs evidence: what did the agent see, what was purchased, what data was delivered. Without an audit log, disputes devolve into he-said-she-said.

2. **Trust engine seeding.** Layer 2 (deferred post-MVP) will compute seller accuracy scores from transaction outcomes. The richer the event history, the stronger the signal. The `buyer_agent_query` and `agent_analysis_summary` fields on `transactions` are a start, but they only capture the purchase endpoint -- not the search/analyze journey that led there.

3. **Sandbox integrity.** The buyer agent platform is the critical isolation boundary. Audit logs are the only mechanism to verify after the fact that an agent stayed within its allowed tool set.

### Scope

Covers: event taxonomy, schema, write path, read path, retention, CLI tooling.
Does NOT cover: real-time alerting, compliance certs (SOC 2), trust engine scoring algorithms.

---

## Design Decisions

### D1: Postgres append-only table, not an external log system

**Choice:** Single `audit_events` table in the existing Postgres 16 instance.

**Trade-offs:**
- **Pro:** Zero new infrastructure. Transactional consistency -- audit event in same DB transaction as the action. Queryable with standard SQL.
- **Pro:** At MVP scale (O(1000) sellers, 10K-100K events/month), Postgres handles this trivially.
- **Con:** If event volume grows to millions/day, migrate to TimescaleDB/ClickHouse.
- **Con:** No built-in streaming. Future consumers need LISTEN/NOTIFY or outbox pattern.

**Scaling trigger:** When `audit_events` exceeds 50M rows or 1K events/second.
### D2: Structured event payload, not free-form text

**Choice:** Fixed envelope (`actor`, `action`, `resource_type`, `resource_id`, `timestamp`) plus typed JSONB `details` field.

**Why:** Free-form logs are cheap to write and expensive to query. Trust engine and dispute resolution need structured filtering/aggregation. JSONB details gives flexibility without schema migrations per event type.

### D3: Synchronous writes in the request path (MVP)

Mutations: audit event in same DB transaction (strong consistency). Reads: fire-and-forget goroutine INSERT (audit failures dont break reads). Post-MVP: async via outbox if latency matters.

### D4: Actor model -- three actor types

| Actor type | Identity source | Example |
|---|---|---|
| `buyer_agent` | `buyer_id` from request header (opaque, from agent platform) | Searches, analyzes, purchases |
| `seller` | `seller_id` from authenticated session (Cognito JWT post-MVP) | Creates/updates listings |
| `operator` | Hardcoded `operator` for CLI/admin actions | Manual onboarding, dispute resolution |

MVP: no auth. buyer_id is unvalidated header. Audit log records whatever identity is presented. Post-MVP: Cognito JWT validation in middleware.

---

## Event Taxonomy

Events organized by resource. Action verbs in past tense.

### Listing events

| Action | Actor | Details |
|---|---|---|
| `listing.created` | seller/operator | seller_id, title, category, price_cents |
| `listing.updated` | seller/operator | changed_fields with old/new values |
| `listing.deleted` | seller/operator | soft_delete: true |
| `listing.data_uploaded` | seller/operator | data_ref, data_format, data_size_bytes |

### Search events

| Action | Actor | Details |
|---|---|---|
| `search.executed` | buyer_agent | query, filters, result_count, top_result_ids (first 10), latency_ms |

**Why log searches:** Search is the product. Feeds relevance tuning (offline nDCG) and trust engine. top_result_ids enables offline evaluation without re-running queries.

### Purchase events

| Action | Actor | Details |
|---|---|---|
| `purchase.initiated` | buyer_agent | listing_id, amount_cents, currency, stripe_payment_id |
| `purchase.confirmed` | buyer_agent | transaction_id, listing_id, ownership_id |
| `purchase.failed` | buyer_agent | transaction_id, listing_id, error_reason |
| `purchase.already_owned` | buyer_agent | listing_id |

### Ownership events

| Action | Actor | Details |
|---|---|---|
| `ownership.download_requested` | buyer_agent | listing_id, data_ref, presigned_url_expiry |

**Why log downloads:** Most sensitive action. Proves what data was served if buyer disputes.

### Buy order events

| Action | Actor | Details |
|---|---|---|
| `buyorder.created` | buyer_agent | query, criteria, max_price_cents, category |
| `buyorder.filled` | seller/operator | listing_id, seller_id |
| `buyorder.cancelled` | buyer_agent/operator | reason |

### Agent sandbox events (future -- from buyer agent platform)

| Action | Actor | Details |
|---|---|---|
| `agent.session_started` | buyer_agent | session_id, buyer_id, original_query |
| `agent.tool_called` | buyer_agent | session_id, tool_name, tool_input_hash, tool_output_summary |
| `agent.session_ended` | buyer_agent | session_id, outcome, duration_ms |
| `agent.data_analyzed` | buyer_agent | session_id, listing_id, questions, analysis_summary |

> **Note:** Emitted by Buyers Agent Platform, not Market Platform. Documented here to establish the shared event schema.

### Operator events

| Action | Actor | Details |
|---|---|---|
| `operator.seller_onboarded` | operator | seller_id, seller_name |
| `operator.refund_processed` | operator | transaction_id, buyer_id, amount_cents, reason |

---

## Schema

### Migration: `002_audit_events.up.sql`

```sql
CREATE TABLE audit_events (
    id            BIGSERIAL PRIMARY KEY,
    timestamp     TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_type    TEXT NOT NULL,
    actor_id      TEXT NOT NULL,
    action        TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id   TEXT,
    details       JSONB NOT NULL DEFAULT '{}',
    request_id    TEXT,
    ip_address    INET
);

CREATE INDEX idx_audit_actor ON audit_events (actor_type, actor_id, timestamp DESC);
CREATE INDEX idx_audit_resource ON audit_events (resource_type, resource_id, timestamp DESC);
CREATE INDEX idx_audit_action ON audit_events (action, timestamp DESC);
CREATE INDEX idx_audit_timestamp ON audit_events (timestamp DESC);
```

### Design notes

- **BIGSERIAL, not UUID:** Append-only and ordered. Efficient for range scans.
- **No foreign keys:** Audit table must never block due to referential integrity.
- **`request_id`:** Correlation ID from X-Request-Id middleware. Joins events from one API call.
- **`ip_address`:** Nullable. For forensics.
- **Immutable:** No updated_at. No update path.

---

## Go Types

**Domain:** `internal/domain/audit.go` -- ActorType constants + AuditEvent struct. Details is `json.RawMessage` (varies by event type).

**Storage:** `internal/storage/postgres/audit_repo.go`
- Write: `Record(ctx, event)` (standalone), `RecordTx(ctx, tx, event)` (transactional)
- Read: `ListByActor`, `ListByResource`, `ListByAction` -- all paginated

---

## Write Path Integration

1. **RequestID middleware** reads/generates `X-Request-Id`, stores in context.
2. **Mutation services** write audit event in same DB transaction via RecordTx.
3. **Read handlers** fire-and-forget Record in goroutine (dont block reads).

---

## Read Path: API

`GET /api/v1/audit-events` with filters: actor_type, actor_id, resource_type, resource_id, action, since, until, limit, offset.

`GET /api/v1/audit-events/{id}` for single event.

No auth in MVP (market platform is internal). Post-MVP: operator role JWT.

---

## CLI Tool

```
ie-admin audit list [--actor-type=X] [--actor-id=X] [--action=X] [--since=X] [--limit=N]
ie-admin audit get <event-id>
ie-admin audit summary --since=24h
ie-admin audit trail <resource-type> <resource-id>
```

`audit trail` is the primary dispute-resolution tool: given a listing or transaction ID, shows every event in chronological order.

---

## Retention

**MVP:** No automatic deletion. < 1GB/year at MVP scale.

**Post-MVP:** Monthly Postgres native partitioning on timestamp. Archive >2yr to S3/MinIO as gzipped JSONL.

---

## Implementation Order

1. Migration + domain type + repo -- the foundation.
2. Request ID middleware -- prerequisite for correlation.
3. Write-path integration -- purchases first (financial transactions), then listings, search, buy orders.
4. Read API + CLI -- the operators interface.
5. Agent sandbox events -- blocked on buyer agent platform design.

---

## Open Questions

- [ ] Should the buyer agent platform write audit events directly to Postgres, or POST to market platform API?
- [ ] Should details JSONB have per-action-type JSON Schema validation? (Recommend: tests only, not runtime.)
- [ ] Should purchase audit events include originating agent session ID? (Depends on purchase flow routing decision.)

---

## Versions

| Dependency | Version | Notes |
|---|---|---|
| PostgreSQL | 16 | Native partitioning for future retention |
| Go | 1.22+ | Context cancellation in goroutines |
| sqlx | latest | Used by existing repos |
| chi v5 | latest | Middleware chain for request ID |

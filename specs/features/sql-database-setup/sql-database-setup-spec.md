# SQL Database Setup — MVP Spec

## Goal

Add structured sample datasets to PostgreSQL so the MVP demo has real,
queryable data behind its listings. Ship a Cobra CLI (`iecli`) that seeds
the database and runs sample SQL queries end-to-end.

## Context

The market-platform already runs Postgres 16 in docker-compose with
schema migrations (001_initial). The REST API and integration tests are
wired up via testcontainers. What's missing:

1. **Sample dataset tables** — structured tables that represent the
   actual *content* sellers are listing for sale (pricing data, ad
   benchmarks, etc.).
2. **Seed data** — demo sellers, listings, buy orders, and rows in the
   dataset tables so `docker compose up && make run` yields a populated
   marketplace.
3. **CLI tooling** — a Go/Cobra CLI to seed data and run sample SQL
   queries, per the project rule that every API ships with a CLI.

## Design

### Migration 002 — Sample Dataset Tables

Three tables representing seller datasets:

| Table | Description |
|-------|-------------|
| `sample_consumer_electronics_pricing` | Per-product pricing across retailers |
| `sample_ecommerce_price_comparison` | Cross-platform (Amazon/Walmart/Target) price comparison |
| `sample_shopping_ads_benchmark` | Ad performance metrics by category and platform |

These tables are *not* referenced by FK from listings — they represent
raw data content that could be served through the object store in
production. For the MVP demo they live in Postgres so we can show
structured SQL queries.

### CLI — `cmd/iecli`

Built with `spf13/cobra`. Two subcommands:

- **`iecli seed`** — Connects to Postgres (via `DATABASE_URL`), inserts
  demo sellers, listings, buy orders, and sample dataset rows.
  Idempotent (uses `ON CONFLICT DO NOTHING` where possible).
- **`iecli query`** — Runs a set of predefined sample SQL queries
  against the sample dataset tables and prints formatted results. Shows
  the kind of analysis a buyer agent would do.

### Makefile targets

- `make cli` — build `iecli`
- `make seed` — run `iecli seed` against local docker Postgres

## Sample Queries (demonstrated by `iecli query`)

1. Average price by category across retailers
2. Amazon vs Walmart price spread
3. Top ad categories by ROAS
4. Products with biggest cross-platform price differences

## Testing

- The sample dataset migration runs automatically via embedded
  migrations (same as 001).
- Integration tests are unaffected — they use testcontainers with clean
  databases; seed data is only inserted by the CLI.
- The CLI itself is exercised manually and in CI via `make seed`.

## Non-goals

- Buyer agent SQL query execution (future — agents will use MCP tools)
- Row-level access control on sample tables
- Production data ingestion pipeline

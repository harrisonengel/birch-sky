# Semantic Search via OpenSearch ML Ingest Pipeline

## Goal

Make embedding-based search work end-to-end on the demo stack using
an open-source general-text embedding model. We assume low ingestion
volume for the foreseeable future, so we do not need a managed
embedding service like AWS Bedrock; instead we run the model
**inside OpenSearch** as part of the ingest pipeline.

## Decision

Use OpenSearch ML Commons + the neural-search plugin (both bundled in
the `opensearchproject/opensearch:3.0.0` image) to deploy a
pre-trained sentence transformer locally, and add a `text_embedding`
ingest processor so every document indexed into the `listings` index
is auto-embedded server-side.

**Model:** `huggingface/sentence-transformers/all-MiniLM-L6-v2`,
version `1.0.2`, format `TORCH_SCRIPT`, dimension **384**.

Reasoning:
- Strong general-purpose English text embedding quality for its size.
- Tiny (~80 MB) → fits comfortably in a single-node demo cluster's
  native memory.
- Fast on CPU; no GPU requirement.
- Shipped as a *pre-trained* model in OpenSearch ML Commons, so we
  do not need to upload, sign, or host any model files ourselves.

**Why an OpenSearch ingest pipeline (vs. embedding in the Go service):**
- Single source of truth — the field map and model id live in one
  place and are visible via `GET /_ingest/pipeline/listings-embed`.
- Simpler app code: the indexer just sends text fields; the
  `text_embedding` processor populates `embedding` automatically.
- Symmetric query side: the `neural` query type embeds the query
  string with the same model, so we never have to keep two
  embedding implementations in sync.

## Architecture

```
                       ┌──────────────────────────────────────┐
 PUT /listings/_doc    │ OpenSearch                           │
 { embedding_text:"…",  │   ingest pipeline `listings-embed`   │
   title, description,  │     └─ text_embedding processor      │
   tags, content_text } │          (model = all-MiniLM-L6-v2)  │
        ──────────────► │          field_map:                  │
                       │            embedding_text → embedding │
                       │   index `listings` (default_pipeline) │
                       │     fields: title, description,       │
                       │             tags, content_text,       │
                       │             embedding (knn_vector)    │
                       └──────────────────────────────────────┘

 POST /listings/_search { neural: { embedding: { query_text } } }
        ──────────────►   ↑ same model embeds query text, kNN over
                            the 384-dim HNSW index.
```

## Application Changes

- `internal/search/mlsetup.go` — bootstraps ML Commons on startup
  (cluster settings → model group → register → deploy → ingest
  pipeline). Idempotent; finds an already-deployed model on warm
  cluster boots.
- `internal/search/mapping.go` — index mapping function takes a
  `defaultPipeline` argument and adds an `embedding_text` field as
  the pipeline source.
- `internal/search/opensearch.go` — engine now has `EnableMLPipeline`,
  `PipelineMode`, and `SemanticSearch` (neural query). When in
  pipeline mode, `IndexListing` joins listing text into
  `embedding_text` and **omits** the `embedding` field; the pipeline
  fills it in.
- `internal/search/indexer.go` — skips client-side embedding when the
  engine is in pipeline mode.
- `internal/service/turn_market.go` — vector / hybrid modes call
  `engine.SemanticSearch` so the same model embeds query and corpus.
- `internal/config/config.go` — new `EMBEDDING_MODE` env var:
  `opensearch` (default), `bedrock`, or `local`.
- `cmd/server/main.go` — wires the chosen embedding mode.
- `cmd/iecli/ml.go` — adds `iecli ml-status` to inspect the model,
  pipeline, and index from the command line.
- `cmd/iecli/main.go` — `iecli enter` learned a `--mode` flag
  (`text` | `vector` | `hybrid`) for end-to-end manual testing.

## Operational Notes

- **OpenSearch heap.** Bumped from 512 MiB → 2 GiB in
  `docker-compose.yml`. The model loads into JVM-managed native memory
  alongside the kNN index; 512 MiB OOMs on deploy.
- **First boot is slow.** Registering + deploying the model can take
  60–90 seconds on a cold container because the model binary has to
  be downloaded from OpenSearch's HuggingFace mirror. Subsequent
  boots reuse the deployed model. The `market-platform` healthcheck
  has a `start_period: 300s` so compose tolerates the cold-boot wait.
- **Idempotency.** Re-running `EnableMLPipeline` is safe: it searches
  for an already-deployed model by name and reuses it, and pipeline /
  cluster settings PUTs are upserts.
- **Index dimension is fixed at create time.** `EmbeddingDimension`
  defaults to `MLModelDimension` (384). Switching `EMBEDDING_MODE` to
  `bedrock` (1024-dim) requires deleting and recreating the
  `listings` index; the dimension is baked into the mapping.
- **Bedrock fallback still works** for production deployments with
  AWS credentials. The engine + indexer read `engine.PipelineMode()`
  to choose between server-side and client-side embedding, so the
  Bedrock path is unchanged.

## How to Verify

1. `make docker-up-infra && make build && ./bin/market-platform` — the
   server logs `embeddings: bootstrapping OpenSearch ML pipeline …`
   on first boot, then `embeddings: OpenSearch ingest pipeline
   (model_id=…)`.
2. `./bin/iecli ml-status` — confirms the model is `DEPLOYED`, the
   `listings-embed` pipeline exists, and the `listings` index has
   `default_pipeline=listings-embed`.
3. `./bin/iecli seed` — creates demo sellers + listings; each upload
   triggers the pipeline, populating the `embedding` field.
4. `./bin/iecli enter --mode vector "satellite imagery for retail"`
   — runs a `neural` query and ranks listings by semantic similarity.
   The "Retail Parking Lot Satellite Imagery" listing should rank
   high even though the query phrasing doesn't share many tokens
   with the title.
5. `./bin/iecli enter --mode hybrid "weekly grocery prices"` — RRF
   merges BM25 + semantic results.

## Future Work

- Pin a model checksum or local model file once we want
  airgapped/reproducible deployments.
- Add an offline relevance benchmark (a small set of query →
  expected-listing pairs) so we can A/B different models without
  guessing.
- Move ingestion to async + bulk when ingestion rate grows enough
  to make synchronous embedding noticeable in p99 listing-create
  latency.

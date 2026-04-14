# search/ — Design

## Hybrid Search Architecture

Buyer agent search quality is the core product. We use hybrid text + vector search from day one.

### Text Search: BM25F via `combined_fields`
- Fields: `title^3`, `description^2`, `tags^2`, `content_text`
- Custom `listing_analyzer`: standard tokenizer → lowercase → stop words → snowball stemmer
- `combined_fields` chosen over `best_fields` for proper BM25F per-field length normalization

### Vector Search: kNN
- Default: 384-dim embeddings from
  `huggingface/sentence-transformers/all-MiniLM-L6-v2`, deployed
  inside OpenSearch via ML Commons and applied as a `text_embedding`
  ingest processor (`listings-embed`). Query text is embedded
  server-side via the `neural` query type using the same model. See
  `mlsetup.go` and `specs/features/semantic-search/opensearch-ml-pipeline.md`.
- Optional: 1024-dim embeddings from AWS Bedrock Titan Embeddings
  v2:0 when `EMBEDDING_MODE=bedrock` and AWS credentials are present.
  In this mode the application embeds both documents and queries and
  uses the `knn` query type instead of `neural`.
- HNSW index with cosine similarity via the lucene engine.

### Fusion: Reciprocal Rank Fusion (RRF)
- Application-side RRF with k=60
- Text and vector queries issued in parallel
- Results merged by RRF score: `score = Σ 1/(k + rank_i)`

### Embedder Selection
The application's embedding backend is selected by the `EMBEDDING_MODE`
env var:
- `opensearch` (default) — server-side embeddings via OpenSearch ML
  Commons. The Go process never embeds anything; the `Embedder`
  interface is unused on the hot path.
- `bedrock` — `BedrockEmbedder` (AWS Bedrock Titan v2). Used when
  AWS credentials are configured and the operator opts in.
- `local` — `LocalEmbedder`, deterministic hash pseudo-embeddings.
  Tests only.

### Content Extraction

The whole point of indexing data is so a buyer agent can find listings by
what's *inside* them, not just by metadata. We err on the side of
extracting too much rather than too little — search relevance is the
product, and storage / write-path latency are the right things to spend
to get it.

- **CSV**: up to `csvExtractMaxLines` (currently **100,000**) rows fed
  into the index per listing. The bufio.Scanner buffer is bumped to 8MiB
  per token so wide rows aren't truncated mid-line. We will tune this
  number up or down based on real index size and offline relevance
  measurements once those are in place.
- **JSON**: key paths + sample string values (depth-limited).
- **Plain text**: full content, truncated at 50KB.

## Open Follow-Ups

- **Ranking & Relevance Pipeline** — capture every search request and the
  results returned alongside a session ID, then expose `Search()` and
  `Judgement()` APIs to buyer agents so we can build offline ranking
  models from real outcomes. Tracked as a follow-up issue.

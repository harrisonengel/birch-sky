# search/ — Design

## Hybrid Search Architecture

Buyer agent search quality is the core product. We use hybrid text + vector search from day one.

### Text Search: BM25F via `combined_fields`
- Fields: `title^3`, `description^2`, `tags^2`, `content_text`
- Custom `listing_analyzer`: standard tokenizer → lowercase → stop words → snowball stemmer
- `combined_fields` chosen over `best_fields` for proper BM25F per-field length normalization

### Vector Search: kNN
- 1024-dim embeddings from AWS Bedrock Titan Embeddings v2:0
- HNSW index with cosine similarity via nmslib engine
- Query text embedded at search time using same model

### Fusion: Reciprocal Rank Fusion (RRF)
- Application-side RRF with k=60
- Text and vector queries issued in parallel
- Results merged by RRF score: `score = Σ 1/(k + rank_i)`

### Embedder Interface
- `BedrockEmbedder`: production, requires AWS credentials
- `LocalEmbedder`: deterministic hash-based pseudo-embeddings for dev/test

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

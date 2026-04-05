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
- CSV: headers + first 50 rows
- JSON: key paths + sample values (depth-limited)
- Plain text: full content, truncated at 50KB

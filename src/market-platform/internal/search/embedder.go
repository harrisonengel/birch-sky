package search

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

const EmbeddingDimension = 1024

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

// BedrockEmbedder calls AWS Bedrock Titan Embeddings v2 to produce 1024-dim vectors.
type BedrockEmbedder struct {
	client  *bedrockruntime.Client
	modelID string
}

func NewBedrockEmbedder(region string) (*BedrockEmbedder, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	client := bedrockruntime.NewFromConfig(cfg)
	return &BedrockEmbedder{
		client:  client,
		modelID: "amazon.titan-embed-text-v2:0",
	}, nil
}

func (e *BedrockEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	payload := map[string]interface{}{
		"inputText": text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	resp, err := e.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     &e.modelID,
		ContentType: strPtr("application/json"),
		Accept:      strPtr("application/json"),
		Body:        body,
	})
	if err != nil {
		return nil, fmt.Errorf("invoke model: %w", err)
	}

	var result struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return result.Embedding, nil
}

func strPtr(s string) *string { return &s }

// LocalEmbedder produces deterministic hash-based pseudo-embeddings for local dev/test.
// Vectors are consistent for the same input but don't capture semantic similarity.
type LocalEmbedder struct{}

func NewLocalEmbedder() *LocalEmbedder {
	return &LocalEmbedder{}
}

func (e *LocalEmbedder) Embed(_ context.Context, text string) ([]float64, error) {
	vec := make([]float64, EmbeddingDimension)
	h := fnv.New64a()

	for i := 0; i < EmbeddingDimension; i++ {
		h.Reset()
		h.Write([]byte(text))
		h.Write([]byte{byte(i), byte(i >> 8)})
		val := float64(h.Sum64()) / float64(math.MaxUint64)
		vec[i] = val*2 - 1 // normalize to [-1, 1]
	}

	// L2-normalize for cosine similarity
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec, nil
}

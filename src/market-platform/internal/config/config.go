package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL    string
	OpenSearchURL  string
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool
	StripeKey      string
	AnthropicKey   string
	AWSRegion      string
	HTTPPort       int
	MCPPort        int
}

func Load() (*Config, error) {
	httpPort, err := intEnv("HTTP_PORT", 8080)
	if err != nil {
		return nil, fmt.Errorf("HTTP_PORT: %w", err)
	}
	mcpPort, err := intEnv("MCP_PORT", 8081)
	if err != nil {
		return nil, fmt.Errorf("MCP_PORT: %w", err)
	}

	return &Config{
		DatabaseURL:    envOrDefault("DATABASE_URL", "postgres://ieuser:iepass@localhost:5432/iemarket?sslmode=disable"),
		OpenSearchURL:  envOrDefault("OPENSEARCH_URL", "http://localhost:9200"),
		MinIOEndpoint:  envOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: envOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey: envOrDefault("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:    envOrDefault("MINIO_BUCKET", "market-data"),
		MinIOUseSSL:    envOrDefault("MINIO_USE_SSL", "false") == "true",
		StripeKey:      envOrDefault("STRIPE_SECRET_KEY", ""),
		AnthropicKey:   envOrDefault("ANTHROPIC_API_KEY", ""),
		AWSRegion:      envOrDefault("AWS_REGION", ""),
		HTTPPort:       httpPort,
		MCPPort:        mcpPort,
	}, nil
}

// HasBedrock returns true if AWS credentials are configured for Bedrock embeddings.
func (c *Config) HasBedrock() bool {
	return c.AWSRegion != ""
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func intEnv(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q", v)
	}
	return n, nil
}

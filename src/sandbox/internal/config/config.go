// Package config loads sandbox-server settings from environment
// variables. Keep it intentionally tiny — every new knob is a new
// failure mode in production.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	HTTPPort   int
	APIKey     string
	WorkerCount int
	QueueBuffer int
}

// Load reads the environment with sensible defaults that allow the
// MVP to run with no configuration at all.
func Load() (*Config, error) {
	port, err := intEnv("SANDBOX_HTTP_PORT", 8090)
	if err != nil {
		return nil, fmt.Errorf("SANDBOX_HTTP_PORT: %w", err)
	}
	workers, err := intEnv("SANDBOX_WORKERS", 2)
	if err != nil {
		return nil, fmt.Errorf("SANDBOX_WORKERS: %w", err)
	}
	buffer, err := intEnv("SANDBOX_QUEUE_BUFFER", 64)
	if err != nil {
		return nil, fmt.Errorf("SANDBOX_QUEUE_BUFFER: %w", err)
	}
	return &Config{
		HTTPPort:    port,
		APIKey:      os.Getenv("SANDBOX_API_KEY"),
		WorkerCount: workers,
		QueueBuffer: buffer,
	}, nil
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

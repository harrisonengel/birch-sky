package helpers

import (
	"context"
	"fmt"
	"testing"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/objectstore"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const TestBucket = "test-market-data"

type TestMinIO struct {
	Store     objectstore.ObjectStore
	Endpoint  string
	Container testcontainers.Container
}

func SetupTestMinIO(t *testing.T) *TestMinIO {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start minio container: %v", err)
	}
	t.Cleanup(func() {
		container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "9000")
	if err != nil {
		t.Fatalf("get container port: %v", err)
	}

	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	store, err := objectstore.NewMinIOStore(endpoint, "minioadmin", "minioadmin", TestBucket, false)
	if err != nil {
		t.Fatalf("create minio store: %v", err)
	}

	return &TestMinIO{Store: store, Endpoint: endpoint, Container: container}
}

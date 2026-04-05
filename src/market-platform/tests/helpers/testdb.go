package helpers

import (
	"context"
	"fmt"
	"testing"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/postgres"
	"github.com/jmoiron/sqlx"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestDB struct {
	DB        *sqlx.DB
	Container testcontainers.Container
}

func SetupTestDB(t *testing.T) *TestDB {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("iemarket_test"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	t.Cleanup(func() {
		container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("get container port: %v", err)
	}

	dsn := fmt.Sprintf("postgres://testuser:testpass@%s:%s/iemarket_test?sslmode=disable", host, port.Port())

	db, err := postgres.Connect(dsn)
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})

	if err := postgres.RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return &TestDB{DB: db, Container: container}
}

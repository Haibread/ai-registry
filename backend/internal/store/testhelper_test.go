package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/haibread/ai-registry/internal/store"
)

// sharedDB is the single DB instance shared across all tests in this package.
// Initialised once in TestMain.
var sharedDB *store.DB

// TestMain starts one Postgres container for the entire test run and tears it
// down when all tests finish. Per-test isolation is achieved by truncating
// tables in resetDB.
func TestMain(m *testing.M) {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("registry_test"),
		postgres.WithUsername("registry"),
		postgres.WithPassword("registry"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		panic("starting postgres container: " + err.Error())
	}
	defer testcontainers.TerminateContainer(ctr) //nolint:errcheck

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic("getting connection string: " + err.Error())
	}
	if err := store.Migrate(dsn); err != nil {
		panic("running migrations: " + err.Error())
	}

	sharedDB, err = store.Open(ctx, dsn, 5, 1)
	if err != nil {
		panic("opening db: " + err.Error())
	}
	defer sharedDB.Close()

	os.Exit(m.Run())
}

// resetDB truncates all data tables in dependency order.
func resetDB(t *testing.T) {
	t.Helper()
	_, err := sharedDB.Pool.Exec(context.Background(),
		`TRUNCATE mcp_server_versions, mcp_servers, publishers RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
}

// insertPublisher is a test helper that creates a publisher row directly.
func insertPublisher(t *testing.T, slug, name string) string {
	t.Helper()
	id := store.NewULID()
	_, err := sharedDB.Pool.Exec(context.Background(),
		`INSERT INTO publishers (id, slug, name) VALUES ($1, $2, $3)`,
		id, slug, name)
	if err != nil {
		t.Fatalf("inserting publisher: %v", err)
	}
	return id
}

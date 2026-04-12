package handlers_test

import (
	"context"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/haibread/ai-registry/internal/store"
)

// testDB is shared across all handler integration tests in this package.
var testDB *store.DB

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

	testDB, err = store.Open(ctx, dsn, 5, 1)
	if err != nil {
		panic("opening db: " + err.Error())
	}
	defer testDB.Close()

	os.Exit(m.Run())
}

// resetTables truncates all data tables between tests.
func resetTables(t *testing.T) {
	t.Helper()
	_, err := testDB.Pool.Exec(context.Background(),
		`TRUNCATE agent_versions, agents, mcp_server_versions, mcp_servers, publishers, audit_log, reports RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
}

// seedPublisher inserts a publisher row directly and returns its ID.
func seedPublisher(t *testing.T, slug, name string) string {
	t.Helper()
	pub, err := testDB.CreatePublisher(context.Background(), store.CreatePublisherParams{
		Slug: slug, Name: name,
	})
	if err != nil {
		t.Fatalf("seedPublisher(%q): %v", slug, err)
	}
	return pub.ID
}

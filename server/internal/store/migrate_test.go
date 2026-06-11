package store_test

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/haibread/ai-registry/internal/store"
)

// TestMigrate_AppliesAllMigrationsToFreshDatabase verifies that Migrate runs
// cleanly against a brand-new (empty) Postgres and that every expected schema
// object exists after the run. It uses its own container rather than
// sharedDB because sharedDB is already migrated by TestMain.
//
// The assertions intentionally check the *core* tables and a small sample of
// columns added by later migrations rather than the entire schema: the goal
// is to catch "a migration file no longer applies cleanly" drift, not to
// re-specify every column.
func TestMigrate_AppliesAllMigrationsToFreshDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: spins up a dedicated postgres container")
	}

	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("migrate_test"),
		postgres.WithUsername("registry"),
		postgres.WithPassword("registry"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("getting connection string: %v", err)
	}

	// First apply: must succeed against an empty database.
	if err := store.Migrate(dsn); err != nil {
		t.Fatalf("first Migrate call failed: %v", err)
	}

	// Open a pool so we can introspect information_schema.
	db, err := store.Open(ctx, dsn, 2, 1)
	if err != nil {
		t.Fatalf("opening pool: %v", err)
	}
	t.Cleanup(db.Close)

	// Core tables that MUST exist after all migrations. If a migration stops
	// creating one of these, something upstream broke the schema chain.
	wantTables := []string{
		"publishers",
		"mcp_servers",
		"mcp_server_versions",
		"agents",
		"agent_versions",
		"audit_log",
		"reports",
		"instance_tags",
	}
	for _, table := range wantTables {
		var exists bool
		err := db.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("checking table %q: %v", table, err)
		}
		if !exists {
			t.Errorf("table %q not created by migrations", table)
		}
	}

	// A sample of columns added by later migrations on mcp_servers. They
	// were added incrementally and prove the later migrations ran in order.
	//   - 000002: featured (its `tags` twin was dropped by 000022)
	//   - 000003: verified
	//   - 000004: readme
	//   - 000005: view_count, copy_count
	wantMCPColumns := []string{
		"featured",
		"verified",
		"readme",
		"view_count",
		"copy_count",
	}
	for _, col := range wantMCPColumns {
		var exists bool
		err := db.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name   = 'mcp_servers'
				  AND column_name  = $1
			)`, col).Scan(&exists)
		if err != nil {
			t.Fatalf("checking column mcp_servers.%s: %v", col, err)
		}
		if !exists {
			t.Errorf("column mcp_servers.%s not created by migrations", col)
		}
	}

	// 000022 moved tags from the entry tables to the version tables: the
	// entry-level column must be gone and the version-level one present.
	// 000023 added the `managed` flag for config-reconciled tags.
	columnExists := func(table, col string) bool {
		var exists bool
		err := db.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name   = $1
				  AND column_name  = $2
			)`, table, col).Scan(&exists)
		if err != nil {
			t.Fatalf("checking column %s.%s: %v", table, col, err)
		}
		return exists
	}
	if columnExists("mcp_servers", "tags") {
		t.Error("column mcp_servers.tags should have been dropped by 000022")
	}
	if columnExists("agents", "tags") {
		t.Error("column agents.tags should have been dropped by 000022")
	}
	if !columnExists("mcp_server_versions", "tags") {
		t.Error("column mcp_server_versions.tags not created by 000022")
	}
	if !columnExists("agent_versions", "tags") {
		t.Error("column agent_versions.tags not created by 000022")
	}
	if !columnExists("instance_tags", "managed") {
		t.Error("column instance_tags.managed not created by 000023")
	}

	// Idempotency: running Migrate a second time must be a no-op (Migrate
	// wraps ErrNoChange and returns nil — if it ever starts returning the
	// raw error this test will catch it).
	if err := store.Migrate(dsn); err != nil {
		t.Fatalf("second Migrate call should be a no-op but failed: %v", err)
	}

	// And a third — proves the check above is not coincidentally returning
	// nil from some non-migration code path.
	if err := store.Migrate(dsn); err != nil {
		t.Fatalf("third Migrate call should be a no-op but failed: %v", err)
	}
}

package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/haibread/ai-registry/internal/store"
	"github.com/haibread/ai-registry/migrations"
)

// TestMigration0011_GateFiresOnNullWorkspaceID verifies the sanity gate in
// 000011_workspaces_finalise.up.sql: applying the migration when any row
// in mcp_servers or agents still has a NULL workspace_id MUST fail with a
// friendly error message, not silently corrupt schema.
//
// The test runs migrations 1..10, hand-seeds a row with NULL workspace_id
// (allowed by 000008 since the column was nullable), then tries to apply
// 000011 and asserts the gate fires.
func TestMigration0011_GateFiresOnNullWorkspaceID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: spins up a dedicated postgres container")
	}

	ctx := context.Background()
	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("migrate_gate_test"),
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
		t.Fatalf("connection string: %v", err)
	}

	// Apply migrations 1..10 only. Steps(10) advances exactly ten versions
	// from the unversioned starting state.
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("iofs source: %v", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src,
		strings.Replace(dsn, "postgres://", "pgx5://", 1))
	if err != nil {
		t.Fatalf("new migrator: %v", err)
	}
	t.Cleanup(func() { _, _ = m.Close() })

	if err := m.Steps(10); err != nil {
		t.Fatalf("applying migrations 1..10: %v", err)
	}

	// Hand-seed a publisher + an mcp_server with NULL workspace_id.
	// (publisher_id is still NOT NULL at this schema version — 000011 is
	// what drops it. workspace_id was added as nullable in 000008.)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	pubID := store.NewULID()
	if _, err := pool.Exec(ctx,
		`INSERT INTO publishers (id, slug, name) VALUES ($1, 'acme', 'Acme')`,
		pubID); err != nil {
		t.Fatalf("seed publisher: %v", err)
	}
	srvID := store.NewULID()
	if _, err := pool.Exec(ctx,
		`INSERT INTO mcp_servers (id, publisher_id, slug, name)
		 VALUES ($1, $2, 'weather', 'Weather')`,
		srvID, pubID); err != nil {
		t.Fatalf("seed mcp_server: %v", err)
	}

	// Attempt to apply 000011. Must fail with the gate's RAISE EXCEPTION.
	stepErr := m.Steps(1)
	if stepErr == nil {
		t.Fatal("expected 000011 to fail with backfill-not-complete gate, got nil")
	}
	if !strings.Contains(stepErr.Error(), "workspace backfill not complete") {
		t.Errorf("expected gate-fire error, got: %v", stepErr)
	}

	// The failed Steps(1) leaves the migration table in a "dirty" state.
	// Close the current migrator, clear dirty via a fresh migrator's
	// Force(10), then heal the data and replay.
	_, _ = m.Close()
	m2, err := migrate.NewWithSourceInstance("iofs", src,
		strings.Replace(dsn, "postgres://", "pgx5://", 1))
	if err != nil {
		t.Fatalf("re-opening migrator: %v", err)
	}
	t.Cleanup(func() { _, _ = m2.Close() })
	if err := m2.Force(10); err != nil {
		t.Fatalf("force version 10: %v", err)
	}
	wsID := store.NewULID()
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspaces (id, publisher_id, slug, name)
		 VALUES ($1, $2, 'default', 'Default workspace')`,
		wsID, pubID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE mcp_servers SET workspace_id = $1 WHERE id = $2`,
		wsID, srvID); err != nil {
		t.Fatalf("backfill workspace_id: %v", err)
	}

	if err := m2.Steps(1); err != nil {
		t.Fatalf("000011 should succeed after backfill, got: %v", err)
	}

	// Schema assertions: publisher_id is gone, workspace_id is NOT NULL.
	var hasPubID bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
		    SELECT 1 FROM information_schema.columns
		    WHERE table_name='mcp_servers' AND column_name='publisher_id'
		)`).Scan(&hasPubID); err != nil {
		t.Fatalf("introspect publisher_id: %v", err)
	}
	if hasPubID {
		t.Error("publisher_id column should be dropped from mcp_servers")
	}

	var nullable string
	if err := pool.QueryRow(ctx, `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_name='mcp_servers' AND column_name='workspace_id'`).Scan(&nullable); err != nil {
		t.Fatalf("introspect workspace_id nullability: %v", err)
	}
	if nullable != "NO" {
		t.Errorf("workspace_id is_nullable = %q, want NO", nullable)
	}
}

// TestMigration0011_RoundTrip applies up → down → up and asserts the down
// migration leaves the schema in a state equivalent to pre-000011 (publisher_id
// restored, workspace_id nullable again), and that re-applying up succeeds.
func TestMigration0011_RoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: spins up a dedicated postgres container")
	}

	ctx := context.Background()
	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("migrate_rt_test"),
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
		t.Fatalf("connection string: %v", err)
	}

	// Apply all migrations including 000011 against an empty DB. The
	// sanity gate trivially passes when there are no rows.
	if err := store.Migrate(dsn); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	src, _ := iofs.New(migrations.FS, ".")
	m, err := migrate.NewWithSourceInstance("iofs", src,
		strings.Replace(dsn, "postgres://", "pgx5://", 1))
	if err != nil {
		t.Fatalf("new migrator: %v", err)
	}
	t.Cleanup(func() { _, _ = m.Close() })

	// 000012 (RBAC) and 000013 (drop workspaces) now sit on top of 000011.
	// Reverse both so this test exercises the 000011 down→up round-trip with
	// 000011 as the effective head — the schema state it was written to assert
	// against. 000013.down recreates workspaces and re-adds workspace_id.
	if err := m.Steps(-2); err != nil {
		t.Fatalf("rolling back 000013+000012 to reach the 000011 head: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	// Seed a publisher + workspace + mcp_server BEFORE the down step so
	// the down migration's `UPDATE … FROM workspaces` has data to copy.
	pubID := store.NewULID()
	if _, err := pool.Exec(ctx,
		`INSERT INTO publishers (id, slug, name) VALUES ($1, 'acme', 'Acme')`,
		pubID); err != nil {
		t.Fatalf("seed publisher: %v", err)
	}
	wsID := store.NewULID()
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspaces (id, publisher_id, slug, name)
		 VALUES ($1, $2, 'default', 'Default workspace')`,
		wsID, pubID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	srvID := store.NewULID()
	if _, err := pool.Exec(ctx,
		`INSERT INTO mcp_servers (id, workspace_id, slug, name)
		 VALUES ($1, $2, 'weather', 'Weather')`,
		srvID, wsID); err != nil {
		t.Fatalf("seed mcp_server: %v", err)
	}

	// Step down once → 000011.down should reinstate publisher_id.
	if err := m.Steps(-1); err != nil {
		t.Fatalf("rolling back 000011: %v", err)
	}

	var gotPubID string
	if err := pool.QueryRow(ctx,
		`SELECT publisher_id FROM mcp_servers WHERE id=$1`, srvID).Scan(&gotPubID); err != nil {
		t.Fatalf("read publisher_id after down: %v", err)
	}
	if gotPubID != pubID {
		t.Errorf("publisher_id after down = %q, want %q", gotPubID, pubID)
	}

	// Step back up → 000011 should re-apply cleanly.
	if err := m.Steps(1); err != nil {
		t.Fatalf("re-applying 000011: %v", err)
	}

	// Final state: publisher_id is gone, workspace_id is NOT NULL, and the
	// unique key is (workspace_id, slug) — not (publisher_id, slug). Verify
	// the constraint names directly so a future migration that quietly
	// renames or rebuilds them gets caught.
	for _, tbl := range []string{"mcp_servers", "agents"} {
		var hasOldUnique bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
			    SELECT 1 FROM information_schema.table_constraints
			    WHERE table_name = $1
			      AND constraint_name = $1 || '_publisher_id_slug_key'
			)`, tbl).Scan(&hasOldUnique); err != nil {
			t.Fatalf("introspect %s old unique: %v", tbl, err)
		}
		if hasOldUnique {
			t.Errorf("%s: old (publisher_id, slug) unique key should be dropped", tbl)
		}
		var hasNewUnique bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
			    SELECT 1 FROM information_schema.table_constraints
			    WHERE table_name = $1
			      AND constraint_name = $1 || '_workspace_id_slug_key'
			)`, tbl).Scan(&hasNewUnique); err != nil {
			t.Fatalf("introspect %s new unique: %v", tbl, err)
		}
		if !hasNewUnique {
			t.Errorf("%s: new (workspace_id, slug) unique key missing", tbl)
		}
	}
}

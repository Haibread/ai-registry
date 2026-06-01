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

// newMigrateContainer spins up a dedicated Postgres and returns a migrator
// plus a connection pool, both registered for cleanup.
func newMigrateContainer(t *testing.T, dbName string) (*migrate.Migrate, *pgxpool.Pool, func() *migrate.Migrate) {
	t.Helper()
	ctx := context.Background()
	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase(dbName),
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

	newMigrator := func() *migrate.Migrate {
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
		return m
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return newMigrator(), pool, newMigrator
}

// TestMigration0013_DropsWorkspaces verifies the finalise migration that
// removes the workspace layer: it backfills publisher_id from the
// workspace link (catching rows that only carry workspace_id), flips
// publisher_id NOT NULL, swaps the slug unique key back to
// (publisher_id, slug), drops workspace_id, and drops the workspaces table.
func TestMigration0013_DropsWorkspaces(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: spins up a dedicated postgres container")
	}

	ctx := context.Background()
	m, pool, _ := newMigrateContainer(t, "migrate_drop_ws_test")

	// Advance to schema version 12 (post-RBAC, pre-drop): resources carry
	// both workspace_id (NOT NULL) and a nullable publisher_id.
	if err := m.Steps(12); err != nil {
		t.Fatalf("applying migrations 1..12: %v", err)
	}

	// Seed publisher + workspace + a server that has workspace_id but a NULL
	// publisher_id — exercises the migration's re-backfill step.
	pubID := store.NewULID()
	if _, err := pool.Exec(ctx,
		`INSERT INTO publishers (id, slug, name) VALUES ($1, 'acme', 'Acme')`,
		pubID); err != nil {
		t.Fatalf("seed publisher: %v", err)
	}
	wsID := store.NewULID()
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspaces (id, publisher_id, slug, name)
		 VALUES ($1, $2, 'default', 'Default')`,
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

	// Apply 000013.
	if err := m.Steps(1); err != nil {
		t.Fatalf("applying 000013: %v", err)
	}

	// publisher_id was backfilled from the workspace and is now NOT NULL.
	var gotPubID string
	if err := pool.QueryRow(ctx,
		`SELECT publisher_id FROM mcp_servers WHERE id = $1`, srvID).Scan(&gotPubID); err != nil {
		t.Fatalf("read publisher_id: %v", err)
	}
	if gotPubID != pubID {
		t.Errorf("backfilled publisher_id = %q, want %q", gotPubID, pubID)
	}
	var nullable string
	if err := pool.QueryRow(ctx, `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_name = 'mcp_servers' AND column_name = 'publisher_id'`).Scan(&nullable); err != nil {
		t.Fatalf("introspect publisher_id nullability: %v", err)
	}
	if nullable != "NO" {
		t.Errorf("publisher_id is_nullable = %q, want NO", nullable)
	}

	// workspace_id column and the workspaces table are gone.
	for _, tbl := range []string{"mcp_servers", "agents"} {
		var hasWorkspaceCol bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
			    SELECT 1 FROM information_schema.columns
			    WHERE table_name = $1 AND column_name = 'workspace_id'
			)`, tbl).Scan(&hasWorkspaceCol); err != nil {
			t.Fatalf("introspect %s.workspace_id: %v", tbl, err)
		}
		if hasWorkspaceCol {
			t.Errorf("%s.workspace_id should be dropped", tbl)
		}
	}
	var hasWorkspacesTable bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
		    SELECT 1 FROM information_schema.tables WHERE table_name = 'workspaces'
		)`).Scan(&hasWorkspacesTable); err != nil {
		t.Fatalf("introspect workspaces table: %v", err)
	}
	if hasWorkspacesTable {
		t.Error("workspaces table should be dropped")
	}

	// Unique key is (publisher_id, slug) again, not (workspace_id, slug).
	for _, tbl := range []string{"mcp_servers", "agents"} {
		var hasNewKey, hasOldKey bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
			    SELECT 1 FROM information_schema.table_constraints
			    WHERE table_name = $1 AND constraint_name = $1 || '_publisher_id_slug_key'
			)`, tbl).Scan(&hasNewKey); err != nil {
			t.Fatalf("introspect %s new key: %v", tbl, err)
		}
		if !hasNewKey {
			t.Errorf("%s: (publisher_id, slug) unique key missing", tbl)
		}
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
			    SELECT 1 FROM information_schema.table_constraints
			    WHERE table_name = $1 AND constraint_name = $1 || '_workspace_id_slug_key'
			)`, tbl).Scan(&hasOldKey); err != nil {
			t.Fatalf("introspect %s old key: %v", tbl, err)
		}
		if hasOldKey {
			t.Errorf("%s: (workspace_id, slug) unique key should be dropped", tbl)
		}
	}
}

// TestMigration0013_GateFiresOnDuplicateSlug verifies gate 2: a publisher with
// two workspaces each exposing a same-slug server would collide on the
// restored (publisher_id, slug) key, so the migration must abort loudly rather
// than silently lose a row.
func TestMigration0013_GateFiresOnDuplicateSlug(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: spins up a dedicated postgres container")
	}

	ctx := context.Background()
	m, pool, _ := newMigrateContainer(t, "migrate_drop_ws_dup_test")

	if err := m.Steps(12); err != nil {
		t.Fatalf("applying migrations 1..12: %v", err)
	}

	pubID := store.NewULID()
	if _, err := pool.Exec(ctx,
		`INSERT INTO publishers (id, slug, name) VALUES ($1, 'acme', 'Acme')`,
		pubID); err != nil {
		t.Fatalf("seed publisher: %v", err)
	}
	// Two workspaces under one publisher, each with a 'weather' server. Both
	// servers carry NULL publisher_id so the re-backfill funnels them to the
	// same (publisher_id, slug) pair.
	for _, ws := range []string{"team-a", "team-b"} {
		wsID := store.NewULID()
		if _, err := pool.Exec(ctx,
			`INSERT INTO workspaces (id, publisher_id, slug, name)
			 VALUES ($1, $2, $3, $3)`,
			wsID, pubID, ws); err != nil {
			t.Fatalf("seed workspace %s: %v", ws, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO mcp_servers (id, workspace_id, slug, name)
			 VALUES ($1, $2, 'weather', 'Weather')`,
			store.NewULID(), wsID); err != nil {
			t.Fatalf("seed server in %s: %v", ws, err)
		}
	}

	stepErr := m.Steps(1)
	if stepErr == nil {
		t.Fatal("expected 000013 to abort on duplicate (publisher_id, slug), got nil")
	}
	if !strings.Contains(stepErr.Error(), "duplicate (publisher_id, slug) in mcp_servers") {
		t.Errorf("expected duplicate-slug gate error, got: %v", stepErr)
	}
}

// TestMigration0013_RoundTrip applies up → down → up against an otherwise
// empty database and asserts the down migration restores the workspaces table
// (with a default workspace per publisher) and that re-applying up succeeds.
func TestMigration0013_RoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: spins up a dedicated postgres container")
	}

	ctx := context.Background()
	m, pool, _ := newMigrateContainer(t, "migrate_drop_ws_rt_test")

	// Up to exactly 000013 (gates trivially pass empty). Pinned to an absolute
	// version so migrations added on top (e.g. 000014 sessions) don't shift the
	// down step below.
	if err := m.Migrate(13); err != nil {
		t.Fatalf("migrating to 000013: %v", err)
	}

	// Seed a publisher + a server BEFORE the down step so the down
	// migration's default-workspace backfill has a row to re-point.
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

	// Down one step → 000013.down recreates workspaces + re-adds workspace_id.
	if err := m.Steps(-1); err != nil {
		t.Fatalf("rolling back 000013: %v", err)
	}

	var hasWorkspacesTable bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
		    SELECT 1 FROM information_schema.tables WHERE table_name = 'workspaces'
		)`).Scan(&hasWorkspacesTable); err != nil {
		t.Fatalf("introspect workspaces table after down: %v", err)
	}
	if !hasWorkspacesTable {
		t.Error("down migration should recreate the workspaces table")
	}

	// The server was re-pointed at its publisher's default workspace.
	var gotWS string
	if err := pool.QueryRow(ctx, `
		SELECT w.slug FROM mcp_servers s
		JOIN workspaces w ON w.id = s.workspace_id
		WHERE s.id = $1`, srvID).Scan(&gotWS); err != nil {
		t.Fatalf("read server workspace after down: %v", err)
	}
	if gotWS != "default" {
		t.Errorf("server workspace after down = %q, want default", gotWS)
	}

	// Re-apply 000013 → must finalise cleanly again.
	if err := m.Steps(1); err != nil {
		t.Fatalf("re-applying 000013: %v", err)
	}
	var hasWorkspaceCol bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
		    SELECT 1 FROM information_schema.columns
		    WHERE table_name = 'mcp_servers' AND column_name = 'workspace_id'
		)`).Scan(&hasWorkspaceCol); err != nil {
		t.Fatalf("introspect workspace_id after re-up: %v", err)
	}
	if hasWorkspaceCol {
		t.Error("re-applied 000013 should drop workspace_id again")
	}
}

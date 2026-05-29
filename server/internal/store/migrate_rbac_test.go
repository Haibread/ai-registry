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

// TestMigration0012_RBAC verifies the additive RBAC migration:
//   - the four RBAC tables are created;
//   - publisher_id is re-added to resources and backfilled from the workspace
//     link;
//   - each workspace group_name binding is converted into a group + an Editor
//     grant on the workspace's publisher.
//
// It applies migrations 1..11 (the post-finalise schema where resources carry
// workspace_id but no publisher_id), seeds a publisher + a workspace with a
// group binding + a server, then applies 000012 and asserts the result.
func TestMigration0012_RBAC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: spins up a dedicated postgres container")
	}

	ctx := context.Background()
	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("migrate_rbac_test"),
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

	// Advance to schema version 11 (post-workspaces-finalise).
	if err := m.Steps(11); err != nil {
		t.Fatalf("applying migrations 1..11: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	// Seed publisher + workspace (with a group binding) + a server. At v11 the
	// server has workspace_id (NOT NULL) and no publisher_id column.
	pubID := store.NewULID()
	if _, err := pool.Exec(ctx,
		`INSERT INTO publishers (id, slug, name) VALUES ($1, 'acme', 'Acme')`,
		pubID); err != nil {
		t.Fatalf("seed publisher: %v", err)
	}
	wsID := store.NewULID()
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspaces (id, publisher_id, slug, name, group_name)
		 VALUES ($1, $2, 'default', 'Default', 'team-alpha')`,
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

	// Apply 000012.
	if err := m.Steps(1); err != nil {
		t.Fatalf("applying 000012: %v", err)
	}

	// RBAC tables exist.
	for _, table := range []string{"users", "groups", "group_members", "role_grants"} {
		var exists bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
			    SELECT 1 FROM information_schema.tables
			    WHERE table_name = $1
			)`, table).Scan(&exists); err != nil {
			t.Fatalf("introspect table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s should exist after 000012", table)
		}
	}

	// publisher_id re-added and backfilled.
	var gotPubID string
	if err := pool.QueryRow(ctx,
		`SELECT publisher_id FROM mcp_servers WHERE id = $1`, srvID).Scan(&gotPubID); err != nil {
		t.Fatalf("read publisher_id: %v", err)
	}
	if gotPubID != pubID {
		t.Errorf("backfilled publisher_id = %q, want %q", gotPubID, pubID)
	}

	// The group binding was converted into a group + Editor grant.
	var groupID string
	if err := pool.QueryRow(ctx,
		`SELECT id FROM groups WHERE slug = 'team-alpha'`).Scan(&groupID); err != nil {
		t.Fatalf("converted group missing: %v", err)
	}

	var role, source, grantPubID string
	if err := pool.QueryRow(ctx, `
		SELECT role, source, publisher_id
		FROM role_grants
		WHERE principal_type = 'group' AND principal_id = $1`,
		groupID).Scan(&role, &source, &grantPubID); err != nil {
		t.Fatalf("converted grant missing: %v", err)
	}
	if role != "editor" {
		t.Errorf("converted grant role = %q, want editor", role)
	}
	if source != "api" {
		t.Errorf("converted grant source = %q, want api (one-time, deletable)", source)
	}
	if grantPubID != pubID {
		t.Errorf("converted grant publisher_id = %q, want %q", grantPubID, pubID)
	}
}

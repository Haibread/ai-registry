package store_test

import (
	"context"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// TestSeedRBAC_ReviewerGroupAndGrant verifies the boot seed creates the
// reviewer group plus a global (all-publishers) Reviewer grant tagged
// source=config, and that re-running is idempotent (no duplicates).
func TestSeedRBAC_ReviewerGroupAndGrant(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	params := store.SeedRBACParams{ReviewerGroupSlug: "registry-reviewers"}
	if err := sharedDB.SeedRBAC(ctx, params); err != nil {
		t.Fatalf("first SeedRBAC: %v", err)
	}
	if err := sharedDB.SeedRBAC(ctx, params); err != nil {
		t.Fatalf("second SeedRBAC: %v", err)
	}

	g, err := sharedDB.GetGroupBySlug(ctx, "registry-reviewers")
	if err != nil {
		t.Fatalf("reviewer group missing: %v", err)
	}

	grants, err := sharedDB.ListGlobalGrants(ctx)
	if err != nil {
		t.Fatalf("ListGlobalGrants: %v", err)
	}
	if len(grants) != 1 {
		t.Fatalf("global grants = %d, want 1 (idempotent seed)", len(grants))
	}
	gr := grants[0]
	if gr.PrincipalID != g.ID || gr.Role != domain.RoleReviewer || gr.Source != domain.GrantSourceConfig {
		t.Errorf("seed grant = %+v, want reviewer/config on the reviewer group", gr)
	}
}

// TestSeedRBAC_BootstrapAdminCreateOnly verifies the bootstrap admin is created
// with is_server_admin and a password, and that re-seeding with a DIFFERENT
// hash never overwrites the existing password (so a rotated password survives
// reboots).
func TestSeedRBAC_BootstrapAdminCreateOnly(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	if err := sharedDB.SeedRBAC(ctx, store.SeedRBACParams{
		BootstrapAdminEmail:        "boss@example.com",
		BootstrapAdminPasswordHash: "$argon2id$original",
	}); err != nil {
		t.Fatalf("seed bootstrap admin: %v", err)
	}

	u, err := sharedDB.GetUserByEmail(ctx, "boss@example.com")
	if err != nil {
		t.Fatalf("bootstrap admin missing: %v", err)
	}
	if !u.IsServerAdmin || !u.HasPassword {
		t.Errorf("bootstrap admin = %+v, want server admin with a password", u)
	}

	// Re-seed with a different hash → must NOT overwrite (create-only).
	if err := sharedDB.SeedRBAC(ctx, store.SeedRBACParams{
		BootstrapAdminEmail:        "boss@example.com",
		BootstrapAdminPasswordHash: "$argon2id$rotated-by-mistake",
	}); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	creds, err := sharedDB.CredentialsByEmail(ctx, "boss@example.com")
	if err != nil {
		t.Fatalf("CredentialsByEmail: %v", err)
	}
	if creds.PasswordHash != "$argon2id$original" {
		t.Errorf("password hash = %q, want the original (seed must not overwrite)", creds.PasswordHash)
	}
}

// TestSeedRBAC_Empty is a no-op when nothing is configured.
func TestSeedRBAC_Empty(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	if err := sharedDB.SeedRBAC(ctx, store.SeedRBACParams{}); err != nil {
		t.Fatalf("empty SeedRBAC should be a no-op, got: %v", err)
	}
	groups, err := sharedDB.ListGroups(ctx, store.ListGroupsParams{})
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("groups = %d, want 0 with no seed config", len(groups))
	}
}

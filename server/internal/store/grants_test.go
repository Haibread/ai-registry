package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// TestEffectiveRoles exercises the authoritative authorization query across
// the principal/scope matrix (ADR 0006 §4): user grants, group grants reached
// via local membership AND via a verbatim claim-slug match, global
// (all-publishers) grants, and the negative case of a grant scoped to a
// different publisher.
func TestEffectiveRoles(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pubA := insertPublisher(t, "acme", "Acme")
	pubB := insertPublisher(t, "globex", "Globex")

	user, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "dev@acme.test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	editors, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "acme-editors", Name: "Acme Editors"})
	if err != nil {
		t.Fatalf("CreateGroup editors: %v", err)
	}
	claimOnly, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "claim-reviewers", Name: "Claim Reviewers"})
	if err != nil {
		t.Fatalf("CreateGroup claimOnly: %v", err)
	}

	// user is a local member of acme-editors (Editor on pubA), NOT of
	// claim-reviewers (Reviewer on pubA) — that one is reached only via claim.
	if err := sharedDB.AddGroupMember(ctx, editors.ID, user.ID); err != nil {
		t.Fatalf("AddGroupMember: %v", err)
	}

	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalUser, PrincipalID: user.ID, PublisherID: pubA, Role: domain.RoleViewer})
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalGroup, PrincipalID: editors.ID, PublisherID: pubA, Role: domain.RoleEditor})
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalGroup, PrincipalID: claimOnly.ID, PublisherID: pubA, Role: domain.RoleReviewer})
	// A grant on pubB only — must never leak into pubA resolution.
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalUser, PrincipalID: user.ID, PublisherID: pubB, Role: domain.RoleAdmin})

	t.Run("user grant + local group membership", func(t *testing.T) {
		held, err := sharedDB.EffectiveRoles(ctx, store.EffectiveRolesParams{
			UserID:      user.ID,
			PublisherID: pubA,
		})
		if err != nil {
			t.Fatalf("EffectiveRoles: %v", err)
		}
		// Viewer (direct) + Editor (local membership). NOT Reviewer (that group
		// is only reachable via claim) and NOT Admin (that grant is on pubB).
		assertRoles(t, held, domain.RoleViewer, domain.RoleEditor)
	})

	t.Run("claim slug adds group's role", func(t *testing.T) {
		held, err := sharedDB.EffectiveRoles(ctx, store.EffectiveRolesParams{
			UserID:          user.ID,
			ClaimGroupSlugs: []string{"claim-reviewers"},
			PublisherID:     pubA,
		})
		if err != nil {
			t.Fatalf("EffectiveRoles: %v", err)
		}
		assertRoles(t, held, domain.RoleViewer, domain.RoleEditor, domain.RoleReviewer)
	})

	t.Run("other publisher's grant does not leak", func(t *testing.T) {
		held, err := sharedDB.EffectiveRoles(ctx, store.EffectiveRolesParams{
			UserID:      user.ID,
			PublisherID: pubB,
		})
		if err != nil {
			t.Fatalf("EffectiveRoles: %v", err)
		}
		// Only the Admin grant scoped to pubB.
		assertRoles(t, held, domain.RoleAdmin)
	})
}

// TestEffectiveRoles_GlobalGrant verifies a publisher_id-NULL grant applies on
// every publisher (this is how the seeded reviewer group works).
func TestEffectiveRoles_GlobalGrant(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pubA := insertPublisher(t, "acme", "Acme")
	pubB := insertPublisher(t, "globex", "Globex")

	reviewers, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "registry-reviewers", Name: "Reviewers"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	// Global (publisher_id == "") Reviewer grant, like the boot seed.
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalGroup, PrincipalID: reviewers.ID, Role: domain.RoleReviewer, Source: domain.GrantSourceConfig})

	for _, pub := range []string{pubA, pubB} {
		held, err := sharedDB.EffectiveRoles(ctx, store.EffectiveRolesParams{
			ClaimGroupSlugs: []string{"registry-reviewers"},
			PublisherID:     pub,
		})
		if err != nil {
			t.Fatalf("EffectiveRoles(%s): %v", pub, err)
		}
		assertRoles(t, held, domain.RoleReviewer)
	}
}

// TestCreateGrant_GlobalDuplicateConflict verifies NULLS NOT DISTINCT makes a
// second identical global grant collide rather than silently duplicating.
func TestCreateGrant_GlobalDuplicateConflict(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	g, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "g", Name: "G"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	params := store.CreateGrantParams{PrincipalType: domain.PrincipalGroup, PrincipalID: g.ID, Role: domain.RoleReviewer}
	if _, err := sharedDB.CreateGrant(ctx, params); err != nil {
		t.Fatalf("first CreateGrant: %v", err)
	}
	if _, err := sharedDB.CreateGrant(ctx, params); !errors.Is(err, store.ErrConflict) {
		t.Errorf("second identical global CreateGrant err = %v, want ErrConflict", err)
	}
}

// TestEnsureGrant_Idempotent verifies the seed path treats a duplicate as
// success.
func TestEnsureGrant_Idempotent(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	g, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "g", Name: "G"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	params := store.CreateGrantParams{PrincipalType: domain.PrincipalGroup, PrincipalID: g.ID, Role: domain.RoleReviewer, Source: domain.GrantSourceConfig}
	if err := sharedDB.EnsureGrant(ctx, params); err != nil {
		t.Fatalf("first EnsureGrant: %v", err)
	}
	if err := sharedDB.EnsureGrant(ctx, params); err != nil {
		t.Fatalf("second EnsureGrant should be a no-op, got: %v", err)
	}
	grants, err := sharedDB.ListGlobalGrants(ctx)
	if err != nil {
		t.Fatalf("ListGlobalGrants: %v", err)
	}
	if len(grants) != 1 {
		t.Errorf("global grants = %d, want 1 (EnsureGrant must not duplicate)", len(grants))
	}
}

// TestListAndDeleteGrant covers per-publisher listing (with the denormalised
// principal label) and deletion.
func TestListAndDeleteGrant(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pub := insertPublisher(t, "acme", "Acme")
	user, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "owner@acme.test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	grant, err := sharedDB.CreateGrant(ctx, store.CreateGrantParams{
		PrincipalType: domain.PrincipalUser, PrincipalID: user.ID, PublisherID: pub, Role: domain.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("CreateGrant: %v", err)
	}

	grants, err := sharedDB.ListGrantsByPublisher(ctx, pub)
	if err != nil {
		t.Fatalf("ListGrantsByPublisher: %v", err)
	}
	if len(grants) != 1 {
		t.Fatalf("publisher grants = %d, want 1", len(grants))
	}
	if grants[0].PrincipalLabel != "owner@acme.test" {
		t.Errorf("principal label = %q, want the user's email", grants[0].PrincipalLabel)
	}

	if err := sharedDB.DeleteGrant(ctx, grant.ID); err != nil {
		t.Fatalf("DeleteGrant: %v", err)
	}
	if err := sharedDB.DeleteGrant(ctx, grant.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("second DeleteGrant err = %v, want ErrNotFound", err)
	}
}

func mustGrant(t *testing.T, p store.CreateGrantParams) {
	t.Helper()
	if _, err := sharedDB.CreateGrant(context.Background(), p); err != nil {
		t.Fatalf("CreateGrant(%+v): %v", p, err)
	}
}

func assertRoles(t *testing.T, held map[domain.Role]bool, want ...domain.Role) {
	t.Helper()
	wantSet := make(map[domain.Role]bool, len(want))
	for _, r := range want {
		wantSet[r] = true
		if !held[r] {
			t.Errorf("missing expected role %q (held: %v)", r, held)
		}
	}
	for r := range held {
		if !wantSet[r] {
			t.Errorf("unexpected role %q (want: %v)", r, want)
		}
	}
}

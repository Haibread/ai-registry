package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// TestEffectiveRoles exercises the authoritative authorization query across
// the principal/scope matrix: user grants, group grants reached
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

// TestEffectivePublisherIDs verifies the mine-scope query: the set
// of publishers a caller holds a grant on, plus the global-grant flag.
func TestEffectivePublisherIDs(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pubA := insertPublisher(t, "acme", "Acme")
	pubB := insertPublisher(t, "globex", "Globex")
	insertPublisher(t, "initech", "Initech") // no grant — must not appear

	user, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "dev@acme.test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	editors, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "globex-editors", Name: "Globex Editors"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := sharedDB.AddGroupMember(ctx, editors.ID, user.ID); err != nil {
		t.Fatalf("AddGroupMember: %v", err)
	}
	// Direct grant on pubA, group grant (via local membership) on pubB.
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalUser, PrincipalID: user.ID, PublisherID: pubA, Role: domain.RoleEditor})
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalGroup, PrincipalID: editors.ID, PublisherID: pubB, Role: domain.RoleEditor})

	t.Run("scoped to granted publishers, no global", func(t *testing.T) {
		ids, hasGlobal, err := sharedDB.EffectivePublisherIDs(ctx, user.ID, nil)
		if err != nil {
			t.Fatalf("EffectivePublisherIDs: %v", err)
		}
		if hasGlobal {
			t.Errorf("hasGlobal = true, want false")
		}
		assertStringSet(t, ids, pubA, pubB)
	})

	t.Run("no grants returns empty, no global", func(t *testing.T) {
		other, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "nobody@acme.test"})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		ids, hasGlobal, err := sharedDB.EffectivePublisherIDs(ctx, other.ID, nil)
		if err != nil {
			t.Fatalf("EffectivePublisherIDs: %v", err)
		}
		if hasGlobal || len(ids) != 0 {
			t.Errorf("got ids=%v hasGlobal=%v, want empty + false", ids, hasGlobal)
		}
	})

	t.Run("global grant sets hasGlobal", func(t *testing.T) {
		g, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "global-admins", Name: "Global"})
		if err != nil {
			t.Fatalf("CreateGroup: %v", err)
		}
		mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalGroup, PrincipalID: g.ID, Role: domain.RoleAdmin})
		// Reached only via a claim slug, not local membership.
		_, hasGlobal, err := sharedDB.EffectivePublisherIDs(ctx, "", []string{"global-admins"})
		if err != nil {
			t.Fatalf("EffectivePublisherIDs: %v", err)
		}
		if !hasGlobal {
			t.Errorf("hasGlobal = false, want true")
		}
	})
}

// TestListGrantsForPrincipal verifies the /me grants projection: per-publisher
// grants carry the publisher slug/name; a global grant comes back with empty
// publisher fields.
func TestListGrantsForPrincipal(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pubA := insertPublisher(t, "acme", "Acme Corp")

	user, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "dev@acme.test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	reviewers, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "registry-reviewers", Name: "Reviewers"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalUser, PrincipalID: user.ID, PublisherID: pubA, Role: domain.RoleEditor})
	// Global reviewer grant reached via claim slug.
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalGroup, PrincipalID: reviewers.ID, Role: domain.RoleReviewer, Source: domain.GrantSourceConfig})

	grants, err := sharedDB.ListGrantsForPrincipal(ctx, user.ID, []string{"registry-reviewers"})
	if err != nil {
		t.Fatalf("ListGrantsForPrincipal: %v", err)
	}
	if len(grants) != 2 {
		t.Fatalf("got %d grants, want 2: %+v", len(grants), grants)
	}

	var sawPubGrant, sawGlobalGrant bool
	for _, g := range grants {
		switch {
		case g.PublisherID == pubA:
			sawPubGrant = true
			if g.Role != domain.RoleEditor || g.PublisherSlug != "acme" || g.PublisherName != "Acme Corp" {
				t.Errorf("publisher grant = %+v, want editor on acme/Acme Corp", g)
			}
		case g.PublisherID == "":
			sawGlobalGrant = true
			if g.Role != domain.RoleReviewer || g.PublisherSlug != "" || g.PublisherName != "" {
				t.Errorf("global grant = %+v, want reviewer with empty publisher fields", g)
			}
		default:
			t.Errorf("unexpected grant: %+v", g)
		}
	}
	if !sawPubGrant || !sawGlobalGrant {
		t.Errorf("missing grants: pub=%v global=%v", sawPubGrant, sawGlobalGrant)
	}
}

// TestListUserAccess verifies the per-user access aggregate: direct grants and
// grants inherited via local group membership are returned with their display
// enrichment, while grants on groups the user is NOT a local member of (claim
// groups) and other users' grants stay out.
func TestListUserAccess(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pubA := insertPublisher(t, "acme", "Acme Corp")

	user, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "dev@acme.test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	other, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "other@acme.test"})
	if err != nil {
		t.Fatalf("CreateUser other: %v", err)
	}
	editors, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "acme-editors", Name: "Acme Editors"})
	if err != nil {
		t.Fatalf("CreateGroup editors: %v", err)
	}
	claimOnly, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "claim-reviewers", Name: "Claim Reviewers"})
	if err != nil {
		t.Fatalf("CreateGroup claimOnly: %v", err)
	}
	if err := sharedDB.AddGroupMember(ctx, editors.ID, user.ID); err != nil {
		t.Fatalf("AddGroupMember: %v", err)
	}

	// In scope: a direct per-publisher grant, a direct global grant, and a
	// group grant via local membership.
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalUser, PrincipalID: user.ID, PublisherID: pubA, Role: domain.RoleViewer})
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalUser, PrincipalID: user.ID, Role: domain.RoleReviewer, Source: domain.GrantSourceConfig})
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalGroup, PrincipalID: editors.ID, PublisherID: pubA, Role: domain.RoleEditor})
	// Out of scope: a claim-only group's grant and another user's grant.
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalGroup, PrincipalID: claimOnly.ID, PublisherID: pubA, Role: domain.RoleAdmin})
	mustGrant(t, store.CreateGrantParams{PrincipalType: domain.PrincipalUser, PrincipalID: other.ID, PublisherID: pubA, Role: domain.RoleAdmin})

	grants, err := sharedDB.ListUserAccess(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserAccess: %v", err)
	}
	if len(grants) != 3 {
		t.Fatalf("got %d grants, want 3: %+v", len(grants), grants)
	}

	var sawDirect, sawGlobal, sawViaGroup bool
	for _, g := range grants {
		switch {
		case g.Via == "direct" && g.PublisherID == pubA:
			sawDirect = true
			if g.Role != domain.RoleViewer || g.PublisherSlug != "acme" || g.PublisherName != "Acme Corp" || g.GroupID != "" {
				t.Errorf("direct grant = %+v, want viewer on acme with no group", g)
			}
		case g.Via == "direct" && g.PublisherID == "":
			sawGlobal = true
			if g.Role != domain.RoleReviewer || g.Source != domain.GrantSourceConfig || g.PublisherSlug != "" {
				t.Errorf("global grant = %+v, want config-sourced reviewer with empty publisher", g)
			}
		case g.Via == "group":
			sawViaGroup = true
			if g.Role != domain.RoleEditor || g.GroupSlug != "acme-editors" || g.GroupName != "Acme Editors" || g.PublisherSlug != "acme" {
				t.Errorf("group grant = %+v, want editor via acme-editors on acme", g)
			}
		default:
			t.Errorf("unexpected grant: %+v", g)
		}
	}
	if !sawDirect || !sawGlobal || !sawViaGroup {
		t.Errorf("missing grants: direct=%v global=%v viaGroup=%v", sawDirect, sawGlobal, sawViaGroup)
	}
}

// assertStringSet checks got contains exactly the want values, order-independent.
func assertStringSet(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d values %v, want %d %v", len(got), got, len(want), want)
	}
	set := make(map[string]bool, len(got))
	for _, v := range got {
		set[v] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("missing %q in %v", w, got)
		}
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

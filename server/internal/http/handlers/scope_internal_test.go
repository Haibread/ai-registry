package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

type fakeScopeStore struct {
	ids    []string
	global bool
}

func (f fakeScopeStore) EffectivePublisherIDs(_ context.Context, _ string, _ []string) ([]string, bool, error) {
	return f.ids, f.global, nil
}

// TestMineScope covers the branching of the mine=true scope resolver: server
// admins and global-grant holders see everything, an author is scoped to their
// granted publishers, a grant-less author resolves to an empty set, and an
// unauthenticated caller is reported as such (so the handler can 401).
func TestMineScope(t *testing.T) {
	adminPrincipal := &auth.Principal{IsServerAdmin: true}

	t.Run("server admin sees all", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), adminPrincipal)
		ids, all, authed, err := mineScope(ctx, fakeScopeStore{ids: []string{"x"}})
		if err != nil || !all || !authed || len(ids) != 0 {
			t.Fatalf("got ids=%v all=%v authed=%v err=%v, want all+authed", ids, all, authed, err)
		}
	})

	t.Run("global grant sees all", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		_, all, authed, err := mineScope(ctx, fakeScopeStore{global: true})
		if err != nil || !all || !authed {
			t.Fatalf("got all=%v authed=%v err=%v, want all+authed", all, authed, err)
		}
	})

	t.Run("author scoped to granted publishers", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		ids, all, authed, err := mineScope(ctx, fakeScopeStore{ids: []string{"p1", "p2"}})
		if err != nil || all || !authed {
			t.Fatalf("got all=%v authed=%v err=%v, want scoped+authed", all, authed, err)
		}
		if len(ids) != 2 {
			t.Errorf("ids = %v, want 2", ids)
		}
	})

	t.Run("author with no grants resolves empty", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		ids, all, authed, err := mineScope(ctx, fakeScopeStore{})
		if err != nil || all || !authed || len(ids) != 0 {
			t.Fatalf("got ids=%v all=%v authed=%v err=%v, want empty+authed", ids, all, authed, err)
		}
	})

	t.Run("unauthenticated reported", func(t *testing.T) {
		_, _, authed, err := mineScope(context.Background(), fakeScopeStore{})
		if err != nil || authed {
			t.Fatalf("got authed=%v err=%v, want not authed", authed, err)
		}
	})
}

// fakePrivateStore drives canViewPrivate without a DB. pubErr surfaces a
// publisher-lookup failure; roles is what EffectiveRoles returns for the
// (single) publisher under test.
type fakePrivateStore struct {
	pubID  string
	pubErr error
	roles  map[domain.Role]bool
}

func (f fakePrivateStore) GetPublisherBySlug(_ context.Context, _ string) (string, error) {
	return f.pubID, f.pubErr
}

func (f fakePrivateStore) EffectiveRoles(_ context.Context, _ store.EffectiveRolesParams) (map[domain.Role]bool, error) {
	return f.roles, nil
}

// TestCanViewPrivate covers the detail-read visibility gate: a Server Admin and
// the owning publisher's own members (any role, viewer-and-up) see private /
// draft entries; an anonymous caller, a member of a different publisher, and an
// unknown publisher all fall back to public-only — so one publisher's private
// data is never exposed to another's.
func TestCanViewPrivate(t *testing.T) {
	adminPrincipal := &auth.Principal{IsServerAdmin: true}

	t.Run("server admin sees private without consulting the store", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), adminPrincipal)
		// pubErr would surface only if the store were consulted; it must not be.
		if !canViewPrivate(ctx, fakePrivateStore{pubErr: errors.New("store must not be called")}, "acme") {
			t.Fatal("server admin should see private")
		}
	})

	t.Run("anonymous gets public-only", func(t *testing.T) {
		st := fakePrivateStore{pubID: "p1", roles: map[domain.Role]bool{domain.RoleViewer: true}}
		if canViewPrivate(context.Background(), st, "acme") {
			t.Fatal("anonymous must not see private")
		}
	})

	t.Run("publisher viewer sees its private entries", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		st := fakePrivateStore{pubID: "p1", roles: map[domain.Role]bool{domain.RoleViewer: true}}
		if !canViewPrivate(ctx, st, "acme") {
			t.Fatal("a publisher viewer should see its private entries")
		}
	})

	t.Run("publisher editor sees private (viewer implied)", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		st := fakePrivateStore{pubID: "p1", roles: map[domain.Role]bool{domain.RoleEditor: true}}
		if !canViewPrivate(ctx, st, "acme") {
			t.Fatal("a publisher editor should see its private entries")
		}
	})

	t.Run("member of a different publisher gets public-only", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		// No roles resolve on the publisher being read.
		st := fakePrivateStore{pubID: "p1", roles: map[domain.Role]bool{}}
		if canViewPrivate(ctx, st, "other") {
			t.Fatal("a non-member must not see another publisher's private entries")
		}
	})

	t.Run("unknown publisher slug → public-only", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		st := fakePrivateStore{pubErr: store.ErrNotFound}
		if canViewPrivate(ctx, st, "ghost") {
			t.Fatal("unknown publisher must yield public-only")
		}
	})
}

// fakeReviewScopeStore drives reviewerScope without a DB.
type fakeReviewScopeStore struct {
	grants []store.PrincipalGrant
	err    error
}

func (f fakeReviewScopeStore) ListGrantsForPrincipal(_ context.Context, _ string, _ []string) ([]store.PrincipalGrant, error) {
	return f.grants, f.err
}

// TestReviewerScope covers the review-queue scope resolver: a Server Admin and a
// global Reviewer grant see every publisher; a per-publisher Reviewer is scoped
// to the publishers they review (non-Reviewer grants are ignored, since only a
// Reviewer approves); an Editor/Admin-only caller reviews nothing (→ 403); and
// an unauthenticated caller is reported as such (→ 401).
func TestReviewerScope(t *testing.T) {
	adminPrincipal := &auth.Principal{IsServerAdmin: true}

	t.Run("server admin sees all", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), adminPrincipal)
		ids, all, authed, err := reviewerScope(ctx, fakeReviewScopeStore{err: errors.New("store must not be called")})
		if err != nil || !all || !authed || len(ids) != 0 {
			t.Fatalf("got ids=%v all=%v authed=%v err=%v, want see-all", ids, all, authed, err)
		}
	})

	t.Run("global reviewer grant sees all", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		// Empty PublisherID = a global (all-publishers) grant.
		st := fakeReviewScopeStore{grants: []store.PrincipalGrant{{Role: domain.RoleReviewer}}}
		_, all, authed, err := reviewerScope(ctx, st)
		if err != nil || !all || !authed {
			t.Fatalf("got all=%v authed=%v err=%v, want see-all", all, authed, err)
		}
	})

	t.Run("per-publisher reviewer scoped to those publishers", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		st := fakeReviewScopeStore{grants: []store.PrincipalGrant{
			{Role: domain.RoleReviewer, PublisherID: "p1"},
			{Role: domain.RoleEditor, PublisherID: "p2"}, // not a reviewer → ignored
			{Role: domain.RoleReviewer, PublisherID: "p3"},
		}}
		ids, all, authed, err := reviewerScope(ctx, st)
		if err != nil || all || !authed {
			t.Fatalf("got all=%v authed=%v err=%v, want scoped", all, authed, err)
		}
		if len(ids) != 2 {
			t.Fatalf("ids = %v, want two reviewer publishers", ids)
		}
	})

	t.Run("editor/admin only reviews nothing", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		st := fakeReviewScopeStore{grants: []store.PrincipalGrant{
			{Role: domain.RoleAdmin, PublisherID: "p1"},
			{Role: domain.RoleEditor, PublisherID: "p2"},
		}}
		ids, all, authed, err := reviewerScope(ctx, st)
		if err != nil || all || !authed || len(ids) != 0 {
			t.Fatalf("got ids=%v all=%v authed=%v, want empty+authed (→403)", ids, all, authed)
		}
	})

	t.Run("unauthenticated reported", func(t *testing.T) {
		_, all, authed, err := reviewerScope(context.Background(), fakeReviewScopeStore{})
		if err != nil || all || authed {
			t.Fatalf("got all=%v authed=%v err=%v, want not authed", all, authed, err)
		}
	})
}

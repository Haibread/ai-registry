package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// fakeRoleStore lets the pure-Go authz tests exercise CheckPublisherRole
// without a database.
type fakeRoleStore struct {
	roles map[domain.Role]bool
	err   error
}

func (f fakeRoleStore) EffectiveRoles(_ context.Context, _ store.EffectiveRolesParams) (map[domain.Role]bool, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.roles, nil
}

func TestIsAuthenticated(t *testing.T) {
	if auth.IsAuthenticated(context.Background()) {
		t.Error("empty context: IsAuthenticated = true, want false")
	}
	withClaims := auth.ContextWithClaims(context.Background(), &auth.KeycloakClaims{})
	if !auth.IsAuthenticated(withClaims) {
		t.Error("claims context: IsAuthenticated = false, want true")
	}
	withPrincipal := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
	if !auth.IsAuthenticated(withPrincipal) {
		t.Error("principal context: IsAuthenticated = false, want true")
	}
}

func TestCheckPublisherRole(t *testing.T) {
	t.Run("server admin satisfies anything", func(t *testing.T) {
		ctx := auth.ContextWithClaims(context.Background(), &auth.KeycloakClaims{
			RealmAccess: auth.RealmAccess{Roles: []string{"admin"}},
		})
		ok, authed, err := auth.CheckPublisherRole(ctx, fakeRoleStore{}, "p1", domain.RoleEditor)
		if err != nil || !ok || !authed {
			t.Fatalf("ok=%v authed=%v err=%v, want ok+authed", ok, authed, err)
		}
	})

	t.Run("editor grant satisfies editor", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		st := fakeRoleStore{roles: map[domain.Role]bool{domain.RoleEditor: true}}
		ok, authed, err := auth.CheckPublisherRole(ctx, st, "p1", domain.RoleEditor)
		if err != nil || !ok || !authed {
			t.Fatalf("ok=%v authed=%v err=%v, want ok+authed", ok, authed, err)
		}
	})

	t.Run("reviewer does not satisfy editor", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		st := fakeRoleStore{roles: map[domain.Role]bool{domain.RoleReviewer: true}}
		ok, authed, err := auth.CheckPublisherRole(ctx, st, "p1", domain.RoleEditor)
		if err != nil || ok || !authed {
			t.Fatalf("ok=%v authed=%v err=%v, want !ok + authed (separation of duties)", ok, authed, err)
		}
	})

	t.Run("unauthenticated reported", func(t *testing.T) {
		ok, authed, err := auth.CheckPublisherRole(context.Background(), fakeRoleStore{}, "p1", domain.RoleEditor)
		if err != nil || ok || authed {
			t.Fatalf("ok=%v authed=%v err=%v, want neither", ok, authed, err)
		}
	})

	t.Run("store error surfaces", func(t *testing.T) {
		ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{UserID: "u1"})
		st := fakeRoleStore{err: errors.New("boom")}
		_, authed, err := auth.CheckPublisherRole(ctx, st, "p1", domain.RoleEditor)
		if err == nil || !authed {
			t.Fatalf("authed=%v err=%v, want authed + error", authed, err)
		}
	})
}

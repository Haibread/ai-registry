package handlers

import (
	"context"
	"testing"

	"github.com/haibread/ai-registry/internal/auth"
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
	adminClaims := &auth.KeycloakClaims{RealmAccess: auth.RealmAccess{Roles: []string{"admin"}}}

	t.Run("server admin sees all", func(t *testing.T) {
		ctx := auth.ContextWithClaims(context.Background(), adminClaims)
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

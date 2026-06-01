package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

type fakeRoleStore struct {
	roles map[domain.Role]bool
	err   error
	last  *store.EffectiveRolesParams
}

func (f *fakeRoleStore) EffectiveRoles(_ context.Context, p store.EffectiveRolesParams) (map[domain.Role]bool, error) {
	f.last = &p
	return f.roles, f.err
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func resolveFixed(_ *http.Request) (string, error) { return "pub-1", nil }

// runGuard wires the middleware around okHandler and returns the status code,
// injecting the given principal and (optionally) issuer kind into the request
// context.
func runGuard(mw func(http.Handler) http.Handler, p *Principal) int {
	h := mw(okHandler())
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/acme/x", nil)
	ctx := r.Context()
	if p != nil {
		ctx = ContextWithPrincipal(ctx, p)
	}
	r = r.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec.Code
}

func TestRequirePublisherRole_ServerAdminBypass(t *testing.T) {
	rs := &fakeRoleStore{roles: map[domain.Role]bool{}} // no publisher roles
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolveFixed)
	if code := runGuard(mw, &Principal{UserID: "u", IsServerAdmin: true}); code != http.StatusNoContent {
		t.Errorf("server admin should pass, got %d", code)
	}
}

func TestRequirePublisherRole_EditorSatisfiesEditor(t *testing.T) {
	rs := &fakeRoleStore{roles: map[domain.Role]bool{domain.RoleEditor: true}}
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolveFixed)
	if code := runGuard(mw, &Principal{UserID: "u"}); code != http.StatusNoContent {
		t.Errorf("editor should satisfy editor, got %d", code)
	}
}

func TestRequirePublisherRole_ReviewerCannotWrite(t *testing.T) {
	rs := &fakeRoleStore{roles: map[domain.Role]bool{domain.RoleReviewer: true}}
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolveFixed)
	if code := runGuard(mw, &Principal{UserID: "u"}); code != http.StatusForbidden {
		t.Errorf("reviewer-only must NOT satisfy editor, got %d", code)
	}
}

func TestRequirePublisherRole_EditorCannotReview(t *testing.T) {
	rs := &fakeRoleStore{roles: map[domain.Role]bool{domain.RoleEditor: true}}
	mw := RequirePublisherRole(rs, domain.RoleReviewer, resolveFixed)
	if code := runGuard(mw, &Principal{UserID: "u"}); code != http.StatusForbidden {
		t.Errorf("editor-only must NOT satisfy reviewer, got %d", code)
	}
}

func TestRequirePublisherRole_NoPrincipal401(t *testing.T) {
	rs := &fakeRoleStore{roles: map[domain.Role]bool{}}
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolveFixed)
	if code := runGuard(mw, nil); code != http.StatusUnauthorized {
		t.Errorf("missing principal should 401, got %d", code)
	}
}

func TestRequirePublisherRole_UnknownPublisher403(t *testing.T) {
	rs := &fakeRoleStore{roles: map[domain.Role]bool{domain.RoleAdmin: true}}
	resolve := func(*http.Request) (string, error) { return "", store.ErrNotFound }
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolve)
	if code := runGuard(mw, &Principal{UserID: "u"}); code != http.StatusForbidden {
		t.Errorf("unknown publisher should 403, got %d", code)
	}
}

func TestRequirePublisherRole_StoreError500(t *testing.T) {
	rs := &fakeRoleStore{err: errors.New("db down")}
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolveFixed)
	if code := runGuard(mw, &Principal{UserID: "u"}); code != http.StatusInternalServerError {
		t.Errorf("store error should 500, got %d", code)
	}
}

// TestRequirePublisherRole_ClaimsFallback covers the cutover path: a federated
// writer with no resolved Principal (token without an email claim) is still
// authorized via its claim groups → the group's grant. It also asserts the
// claim groups (not a principal's) are what reach EffectiveRoles.
func TestRequirePublisherRole_ClaimsFallback(t *testing.T) {
	rs := &fakeRoleStore{roles: map[domain.Role]bool{domain.RoleEditor: true}}
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolveFixed)

	h := mw(okHandler())
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/acme/x", nil)
	// Claims only — no Principal (the unprovisioned federated writer).
	ctx := ContextWithClaims(r.Context(), &KeycloakClaims{Groups: []string{"anthropic-core"}})
	r = r.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("claim-group writer should pass via the group grant, got %d", rec.Code)
	}
	if rs.last == nil || len(rs.last.ClaimGroupSlugs) != 1 || rs.last.ClaimGroupSlugs[0] != "anthropic-core" {
		t.Errorf("EffectiveRoles should receive the claim groups, got %+v", rs.last)
	}
	if rs.last.UserID != "" {
		t.Errorf("no Principal → UserID should be empty, got %q", rs.last.UserID)
	}
}

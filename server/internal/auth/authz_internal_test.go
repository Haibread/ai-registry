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
}

func (f fakeRoleStore) EffectiveRoles(context.Context, store.EffectiveRolesParams) (map[domain.Role]bool, error) {
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
	rs := fakeRoleStore{roles: map[domain.Role]bool{}} // no publisher roles
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolveFixed)
	if code := runGuard(mw, &Principal{UserID: "u", IsServerAdmin: true}); code != http.StatusNoContent {
		t.Errorf("server admin should pass, got %d", code)
	}
}

func TestRequirePublisherRole_EditorSatisfiesEditor(t *testing.T) {
	rs := fakeRoleStore{roles: map[domain.Role]bool{domain.RoleEditor: true}}
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolveFixed)
	if code := runGuard(mw, &Principal{UserID: "u"}); code != http.StatusNoContent {
		t.Errorf("editor should satisfy editor, got %d", code)
	}
}

func TestRequirePublisherRole_ReviewerCannotWrite(t *testing.T) {
	rs := fakeRoleStore{roles: map[domain.Role]bool{domain.RoleReviewer: true}}
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolveFixed)
	if code := runGuard(mw, &Principal{UserID: "u"}); code != http.StatusForbidden {
		t.Errorf("reviewer-only must NOT satisfy editor, got %d", code)
	}
}

func TestRequirePublisherRole_EditorCannotReview(t *testing.T) {
	rs := fakeRoleStore{roles: map[domain.Role]bool{domain.RoleEditor: true}}
	mw := RequirePublisherRole(rs, domain.RoleReviewer, resolveFixed)
	if code := runGuard(mw, &Principal{UserID: "u"}); code != http.StatusForbidden {
		t.Errorf("editor-only must NOT satisfy reviewer, got %d", code)
	}
}

func TestRequirePublisherRole_NoPrincipal401(t *testing.T) {
	rs := fakeRoleStore{roles: map[domain.Role]bool{}}
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolveFixed)
	if code := runGuard(mw, nil); code != http.StatusUnauthorized {
		t.Errorf("missing principal should 401, got %d", code)
	}
}

func TestRequirePublisherRole_UnknownPublisher403(t *testing.T) {
	rs := fakeRoleStore{roles: map[domain.Role]bool{domain.RoleAdmin: true}}
	resolve := func(*http.Request) (string, error) { return "", store.ErrNotFound }
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolve)
	if code := runGuard(mw, &Principal{UserID: "u"}); code != http.StatusForbidden {
		t.Errorf("unknown publisher should 403, got %d", code)
	}
}

func TestRequirePublisherRole_StoreError500(t *testing.T) {
	rs := fakeRoleStore{err: errors.New("db down")}
	mw := RequirePublisherRole(rs, domain.RoleEditor, resolveFixed)
	if code := runGuard(mw, &Principal{UserID: "u"}); code != http.StatusInternalServerError {
		t.Errorf("store error should 500, got %d", code)
	}
}

func TestRejectLocalToken(t *testing.T) {
	h := RejectLocalToken(okHandler())

	// Local token → rejected.
	r := httptest.NewRequest(http.MethodPost, "/v0/servers/x/versions/1", nil)
	r = r.WithContext(context.WithValue(r.Context(), issuerKindKey, IssuerLocal))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Errorf("local token on MCP surface should 403, got %d", rec.Code)
	}

	// OIDC token → allowed.
	r = httptest.NewRequest(http.MethodPost, "/v0/servers/x/versions/1", nil)
	r = r.WithContext(context.WithValue(r.Context(), issuerKindKey, IssuerOIDC))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusNoContent {
		t.Errorf("OIDC token should pass the MCP wall, got %d", rec.Code)
	}

	// No token → passes (public reads are unaffected; other guards apply).
	r = httptest.NewRequest(http.MethodGet, "/v0/servers", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusNoContent {
		t.Errorf("unauthenticated request should pass the MCP wall, got %d", rec.Code)
	}
}

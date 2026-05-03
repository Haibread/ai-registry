package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haibread/ai-registry/internal/auth"
)

func TestRequireAdmin_NoToken(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()

	auth.RequireAdmin(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if called {
		t.Error("next handler should not have been called")
	}
}

func TestRequireAdmin_NonAdminClaims(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	claims := &auth.KeycloakClaims{RealmAccess: auth.RealmAccess{Roles: []string{"viewer"}}}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(auth.ContextWithClaims(req.Context(), claims))
	rec := httptest.NewRecorder()

	auth.RequireAdmin(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if called {
		t.Error("next handler should not have been called")
	}
}

func TestRequireAdmin_AdminClaims(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	claims := &auth.KeycloakClaims{RealmAccess: auth.RealmAccess{Roles: []string{"admin"}}}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(auth.ContextWithClaims(req.Context(), claims))
	rec := httptest.NewRecorder()

	auth.RequireAdmin(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !called {
		t.Error("next handler should have been called")
	}
}

// ── RequireWorkspaceWrite (ADR 0002) ───────────────────────────────────────

func runRequireWorkspaceWrite(t *testing.T, claims *auth.KeycloakClaims, group string, lookupErr error) (status int, called bool) {
	t.Helper()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	lookup := func(*http.Request) (string, error) { return group, lookupErr }
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if claims != nil {
		req = req.WithContext(auth.ContextWithClaims(req.Context(), claims))
	}
	rec := httptest.NewRecorder()
	auth.RequireWorkspaceWrite(lookup)(next).ServeHTTP(rec, req)
	return rec.Code, called
}

func TestRequireWorkspaceWrite_NoTokenIs401(t *testing.T) {
	status, called := runRequireWorkspaceWrite(t, nil, "claude-team", nil)
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", status)
	}
	if called {
		t.Error("handler should not have been called")
	}
}

func TestRequireWorkspaceWrite_AdminPasses(t *testing.T) {
	claims := &auth.KeycloakClaims{
		RealmAccess: auth.RealmAccess{Roles: []string{"admin"}},
	}
	status, called := runRequireWorkspaceWrite(t, claims, "", nil)
	if status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}
	if !called {
		t.Error("handler should have been called")
	}
}

func TestRequireWorkspaceWrite_GroupMatchPasses(t *testing.T) {
	claims := &auth.KeycloakClaims{
		Groups: []string{"other-team", "claude-team"},
	}
	status, called := runRequireWorkspaceWrite(t, claims, "claude-team", nil)
	if status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}
	if !called {
		t.Error("handler should have been called")
	}
}

func TestRequireWorkspaceWrite_GroupMismatchIs403(t *testing.T) {
	claims := &auth.KeycloakClaims{
		Groups: []string{"other-team"},
	}
	status, called := runRequireWorkspaceWrite(t, claims, "claude-team", nil)
	if status != http.StatusForbidden {
		t.Errorf("status = %d, want 403", status)
	}
	if called {
		t.Error("handler should not have been called")
	}
}

func TestRequireWorkspaceWrite_NullGroupIs403ForNonAdmin(t *testing.T) {
	// A workspace with NULL/empty group_name is admin-only. Non-admins
	// must not be allowed in just because they happen to be in some group.
	claims := &auth.KeycloakClaims{
		Groups: []string{"any-group"},
	}
	status, called := runRequireWorkspaceWrite(t, claims, "", nil)
	if status != http.StatusForbidden {
		t.Errorf("status = %d, want 403", status)
	}
	if called {
		t.Error("handler should not have been called")
	}
}

func TestRequireWorkspaceWrite_LookupErrorIs500(t *testing.T) {
	claims := &auth.KeycloakClaims{Groups: []string{"claude-team"}}
	lookupErr := http.ErrAbortHandler // any non-nil error
	status, called := runRequireWorkspaceWrite(t, claims, "", lookupErr)
	if status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", status)
	}
	if called {
		t.Error("handler should not have been called")
	}
}

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

package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// ── RequireReviewer ─────────────────────────────────────────────────────

func runRequireReviewer(t *testing.T, claims *auth.KeycloakClaims, configuredGroup string) (status int, called bool) {
	t.Helper()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if claims != nil {
		req = req.WithContext(auth.ContextWithClaims(req.Context(), claims))
	}
	rec := httptest.NewRecorder()
	auth.RequireReviewer(configuredGroup)(next).ServeHTTP(rec, req)
	return rec.Code, called
}

func TestRequireReviewer_NoTokenIs401(t *testing.T) {
	status, called := runRequireReviewer(t, nil, "registry-reviewers")
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", status)
	}
	if called {
		t.Error("handler should not be called")
	}
}

func TestRequireReviewer_AdminAlwaysPasses(t *testing.T) {
	claims := &auth.KeycloakClaims{
		RealmAccess: auth.RealmAccess{Roles: []string{"admin"}},
	}
	// Empty configured group: admin still passes.
	status, called := runRequireReviewer(t, claims, "")
	if status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}
	if !called {
		t.Error("handler should be called")
	}
}

func TestRequireReviewer_GroupMatchPasses(t *testing.T) {
	claims := &auth.KeycloakClaims{
		Groups: []string{"registry-reviewers"},
	}
	status, called := runRequireReviewer(t, claims, "registry-reviewers")
	if status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}
	if !called {
		t.Error("handler should be called")
	}
}

func TestRequireReviewer_GroupMismatchIs403(t *testing.T) {
	claims := &auth.KeycloakClaims{
		Groups: []string{"some-other-group"},
	}
	status, called := runRequireReviewer(t, claims, "registry-reviewers")
	if status != http.StatusForbidden {
		t.Errorf("status = %d, want 403", status)
	}
	if called {
		t.Error("handler should not be called")
	}
}

func TestRequireReviewer_EmptyConfiguredGroupRejectsNonAdmin(t *testing.T) {
	// When operators leave AUTH_REVIEWER_GROUP empty, the workflow
	// closes — only admins can approve / reject. Group matching is
	// disabled regardless of what the JWT carries.
	claims := &auth.KeycloakClaims{
		Groups: []string{""}, // pathological but worth defending against
	}
	status, called := runRequireReviewer(t, claims, "")
	if status != http.StatusForbidden {
		t.Errorf("status = %d, want 403", status)
	}
	if called {
		t.Error("handler should not be called")
	}
}

// ── 403 detail messages ────────────────────────────────────────────────────
//
// The detail field is what an operator reads when debugging an
// unexpected 403. These tests pin the wording so a future edit that
// degrades it fails CI.

func TestRequireReviewer_403_NamesGroupOrFlagsDisabled(t *testing.T) {
	run := func(t *testing.T, claims *auth.KeycloakClaims, group string) (int, string) {
		t.Helper()
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		if claims != nil {
			req = req.WithContext(auth.ContextWithClaims(req.Context(), claims))
		}
		rec := httptest.NewRecorder()
		auth.RequireReviewer(group)(next).ServeHTTP(rec, req)
		var body struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body.Detail
	}

	t.Run("empty reviewer group says workflow is admin-only", func(t *testing.T) {
		claims := &auth.KeycloakClaims{Groups: []string{"anything"}}
		status, detail := run(t, claims, "")
		if status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", status)
		}
		if !strings.Contains(detail, "admin-only") {
			t.Errorf("detail should explain that reviewing is admin-only; got %q", detail)
		}
	})

	t.Run("mismatch names the required reviewer group", func(t *testing.T) {
		claims := &auth.KeycloakClaims{Groups: []string{"unrelated"}}
		status, detail := run(t, claims, "registry-reviewers")
		if status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", status)
		}
		if !strings.Contains(detail, `"registry-reviewers"`) {
			t.Errorf("detail should quote the required reviewer group; got %q", detail)
		}
	})
}

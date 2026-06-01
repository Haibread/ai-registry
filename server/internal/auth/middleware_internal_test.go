package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haibread/ai-registry/internal/store"
)

func setCookieByName(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestAuthenticate_NilDeps_PassThrough(t *testing.T) {
	a := NewAuthenticator(nil, nil)
	called := false
	a.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !called {
		t.Fatal("nil sessions/store should pass through to the next handler")
	}
}

func TestAuthenticate_NoCookie_Unauthenticated(t *testing.T) {
	sm, _ := testManager()
	a := NewAuthenticator(sm, newFakePrincipalStore())

	var sawPrincipal bool
	a.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, sawPrincipal = PrincipalFromContext(r.Context())
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if sawPrincipal {
		t.Fatal("no cookie should yield no principal")
	}
}

func TestAuthenticate_ValidSession_SetsPrincipal(t *testing.T) {
	sm, _ := testManager()
	ps := newFakePrincipalStore()
	ps.byID["u1"] = &store.User{ID: "u1", Email: "a@b.com", IsServerAdmin: true}
	a := NewAuthenticator(sm, ps)

	token, err := sm.Issue(context.Background(), IssueParams{
		UserID: "u1", AuthMethod: "oidc", ClaimGroups: []string{"g1"},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	var princ *Principal
	var isAdmin bool
	h := a.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		princ, _ = PrincipalFromContext(r.Context())
		isAdmin = IsServerAdminFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "sess", Value: token})
	h.ServeHTTP(httptest.NewRecorder(), req)

	if princ == nil || princ.UserID != "u1" || princ.Email != "a@b.com" || princ.AuthMethod != "oidc" {
		t.Fatalf("principal not set correctly: %+v", princ)
	}
	if !princ.IsServerAdmin || !isAdmin {
		t.Fatal("server-admin flag from the user row should reflect in the principal + context")
	}
	if len(princ.ClaimGroups) != 1 || princ.ClaimGroups[0] != "g1" {
		t.Fatalf("claim groups not propagated: %+v", princ.ClaimGroups)
	}
}

func TestAuthenticate_StaleCookie_ClearsAndProceeds(t *testing.T) {
	sm, _ := testManager()
	a := NewAuthenticator(sm, newFakePrincipalStore())

	var sawPrincipal bool
	h := a.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, sawPrincipal = PrincipalFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "sess", Value: "no-such-token"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if sawPrincipal {
		t.Fatal("an unknown session must not set a principal")
	}
	if c := setCookieByName(rec, "sess"); c == nil || c.MaxAge >= 0 {
		t.Fatalf("stale cookie should be cleared, got %+v", c)
	}
}

func TestAuthenticate_DisabledUser_403(t *testing.T) {
	sm, _ := testManager()
	ps := newFakePrincipalStore()
	ps.byID["u1"] = &store.User{ID: "u1", Email: "a@b.com", Disabled: true}
	a := NewAuthenticator(sm, ps)

	token, err := sm.Issue(context.Background(), IssueParams{UserID: "u1", AuthMethod: "local"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	called := false
	h := a.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "sess", Value: token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("disabled account should get 403, got %d", rec.Code)
	}
	if called {
		t.Fatal("next handler must not run for a disabled account")
	}
}

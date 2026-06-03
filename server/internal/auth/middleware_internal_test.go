package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testAuthority(t *testing.T) *TokenAuthority {
	t.Helper()
	ta, _, err := NewTokenAuthority("", "", "test-issuer", time.Minute)
	if err != nil {
		t.Fatalf("NewTokenAuthority: %v", err)
	}
	return ta
}

func TestAuthenticate_NilAuthority_PassThrough(t *testing.T) {
	a := NewAuthenticator(nil)
	called := false
	a.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !called {
		t.Fatal("nil authority should pass through to the next handler")
	}
}

func TestAuthenticate_NoHeader_Unauthenticated(t *testing.T) {
	a := NewAuthenticator(testAuthority(t))

	var sawPrincipal bool
	a.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, sawPrincipal = PrincipalFromContext(r.Context())
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if sawPrincipal {
		t.Fatal("no Authorization header should yield no principal")
	}
}

func TestAuthenticate_ValidToken_SetsPrincipal(t *testing.T) {
	ta := testAuthority(t)
	a := NewAuthenticator(ta)

	token, _, err := ta.Mint(MintParams{
		UserID: "u1", Email: "a@b.com", Groups: []string{"g1"}, SrvAdmin: true, AuthMethod: "oidc",
	})
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	var princ *Principal
	var isAdmin bool
	h := a.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		princ, _ = PrincipalFromContext(r.Context())
		isAdmin = IsServerAdminFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if princ == nil || princ.UserID != "u1" || princ.Email != "a@b.com" || princ.AuthMethod != "oidc" {
		t.Fatalf("principal not set correctly: %+v", princ)
	}
	if !princ.IsServerAdmin || !isAdmin {
		t.Fatal("server-admin flag from the token should reflect in the principal + context")
	}
	if len(princ.ClaimGroups) != 1 || princ.ClaimGroups[0] != "g1" {
		t.Fatalf("claim groups not propagated: %+v", princ.ClaimGroups)
	}
}

func TestAuthenticate_InvalidToken_ProceedsUnauthenticated(t *testing.T) {
	a := NewAuthenticator(testAuthority(t))

	var sawPrincipal bool
	h := a.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, sawPrincipal = PrincipalFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if sawPrincipal {
		t.Fatal("an invalid token must not set a principal")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("invalid token should proceed unauthenticated (200), got %d", rec.Code)
	}
}

func TestAuthority_RejectsForeignToken(t *testing.T) {
	// A token minted by a different authority (different key) must not verify.
	other := testAuthority(t)
	token, _, err := other.Mint(MintParams{UserID: "u1", AuthMethod: "local"})
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if _, err := testAuthority(t).Verify(token); err == nil {
		t.Fatal("a token signed by a foreign key must not verify")
	}
}

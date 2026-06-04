package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// stubServiceVerifier is a ServiceTokenVerifier whose result is fixed by the
// test. recordedCall flips when VerifyServiceToken is invoked, so a test can
// assert the IdP path is NOT consulted for a valid registry token.
type stubServiceVerifier struct {
	id           *ServiceIdentity
	err          error
	recordedCall bool
}

func (s *stubServiceVerifier) VerifyServiceToken(context.Context, string) (*ServiceIdentity, error) {
	s.recordedCall = true
	return s.id, s.err
}

func testAuthority(t *testing.T) *TokenAuthority {
	t.Helper()
	ta, _, err := NewTokenAuthority("", "", "test-issuer", time.Minute)
	if err != nil {
		t.Fatalf("NewTokenAuthority: %v", err)
	}
	return ta
}

func TestAuthenticate_NilAuthority_PassThrough(t *testing.T) {
	a := NewAuthenticator(nil, nil)
	called := false
	a.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !called {
		t.Fatal("nil authority should pass through to the next handler")
	}
}

func TestAuthenticate_NoHeader_Unauthenticated(t *testing.T) {
	a := NewAuthenticator(testAuthority(t), nil)

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
	a := NewAuthenticator(ta, nil)

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
	a := NewAuthenticator(testAuthority(t), nil)

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

func TestAuthenticate_ServiceToken_SetsAdminPrincipal(t *testing.T) {
	v := &stubServiceVerifier{id: &ServiceIdentity{
		Subject: "svc-1", Email: "op@x", Groups: []string{"g"}, IsAdmin: true,
	}}
	a := NewAuthenticator(testAuthority(t), v)

	var princ *Principal
	var isAdmin bool
	var claimSubject string
	h := a.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		princ, _ = PrincipalFromContext(r.Context())
		isAdmin = IsServerAdminFromContext(r.Context())
		if c, ok := ClaimsFromContext(r.Context()); ok && c != nil {
			claimSubject = c.Subject
		}
	}))
	// Not a registry token, so registry Verify fails and the IdP path takes over.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer some-idp-token")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !v.recordedCall {
		t.Fatal("a non-registry token should be offered to the service-token verifier")
	}
	if princ == nil || princ.UserID != "svc-1" || princ.Email != "op@x" || princ.AuthMethod != "oidc" {
		t.Fatalf("principal not set from service identity: %+v", princ)
	}
	if !princ.IsServerAdmin || !isAdmin {
		t.Fatal("realm-admin service token should confer Server Admin")
	}
	if claimSubject != "svc-1" {
		t.Fatalf("audit subject should propagate from the service identity, got %q", claimSubject)
	}
}

func TestAuthenticate_ServiceToken_VerifierError_Unauthenticated(t *testing.T) {
	v := &stubServiceVerifier{err: errors.New("invalid token")}
	a := NewAuthenticator(testAuthority(t), v)

	var sawPrincipal bool
	h := a.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, sawPrincipal = PrincipalFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer some-idp-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if sawPrincipal {
		t.Fatal("a rejected service token must not set a principal")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("rejected service token should proceed unauthenticated (200), got %d", rec.Code)
	}
}

func TestAuthenticate_RegistryToken_DoesNotConsultVerifier(t *testing.T) {
	ta := testAuthority(t)
	// Verifier would (wrongly) grant admin if consulted; a valid non-admin
	// registry token must never reach it.
	v := &stubServiceVerifier{id: &ServiceIdentity{Subject: "svc", IsAdmin: true}}
	a := NewAuthenticator(ta, v)

	token, _, err := ta.Mint(MintParams{UserID: "u1", AuthMethod: "local"})
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	var princ *Principal
	h := a.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		princ, _ = PrincipalFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if v.recordedCall {
		t.Fatal("a valid registry token must not be offered to the service-token verifier")
	}
	if princ == nil || princ.UserID != "u1" || princ.IsServerAdmin {
		t.Fatalf("registry principal should win unchanged: %+v", princ)
	}
}

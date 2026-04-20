// Black-box tests for Authenticate middleware, IsAdminFromContext, bearerToken
// (tested indirectly), and Config.JWKSEndpoint.
package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/haibread/ai-registry/internal/auth"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	return priv
}

func bigIntToBase64URL(n *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(n.Bytes())
}

func intToBase64URL(e int) string {
	return base64.RawURLEncoding.EncodeToString(big.NewInt(int64(e)).Bytes())
}

// jwksEntry is a minimal JWK key object for JSON serialisation in tests.
type jwksEntry struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksDoc struct {
	Keys []jwksEntry `json:"keys"`
}

// newFakeJWKSServer starts an httptest server that always returns the public
// key encoded as a JWKS.
func newFakeJWKSServer(t *testing.T, priv *rsa.PrivateKey, kid string) *httptest.Server {
	t.Helper()
	doc := jwksDoc{
		Keys: []jwksEntry{{
			Kty: "RSA",
			Kid: kid,
			Use: "sig",
			N:   bigIntToBase64URL(priv.PublicKey.N),
			E:   intToBase64URL(priv.PublicKey.E),
		}},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
}

// signJWT creates and signs a JWT with the given private key, kid, issuer, and roles.
func signJWT(t *testing.T, priv *rsa.PrivateKey, kid, issuer string, roles []string) string {
	t.Helper()
	return signJWTWithAudience(t, priv, kid, issuer, "", roles)
}

// signJWTWithAudience signs a JWT with an explicit `aud` claim. Pass audience=""
// to omit the claim entirely.
func signJWTWithAudience(t *testing.T, priv *rsa.PrivateKey, kid, issuer, audience string, roles []string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss": issuer,
		"sub": "user-123",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
		"realm_access": map[string]interface{}{
			"roles": roles,
		},
	}
	if audience != "" {
		claims["aud"] = audience
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid

	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("signing JWT: %v", err)
	}
	return signed
}

// buildValidator creates an auth.Validator backed by the given JWKS server URL.
func buildValidator(t *testing.T, jwksURL, issuer string) *auth.Validator {
	t.Helper()
	cache := auth.NewJWKSCache(jwksURL, time.Minute)
	return auth.NewValidator(cache, issuer, "")
}

// buildValidatorWithAudience is like buildValidator but also enforces a
// required JWT audience claim.
func buildValidatorWithAudience(t *testing.T, jwksURL, issuer, audience string) *auth.Validator {
	t.Helper()
	cache := auth.NewJWKSCache(jwksURL, time.Minute)
	return auth.NewValidator(cache, issuer, audience)
}

// ---------------------------------------------------------------------------
// Authenticate middleware tests
// ---------------------------------------------------------------------------

func TestAuthenticate_NoAuthorizationHeader(t *testing.T) {
	priv := generateTestKey(t)
	const kid = "k1"
	const issuer = "http://keycloak/realms/test"

	jwksSrv := newFakeJWKSServer(t, priv, kid)
	defer jwksSrv.Close()

	v := buildValidator(t, jwksSrv.URL, issuer)

	var capturedCtx context.Context
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	v.Authenticate(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if claims, ok := auth.ClaimsFromContext(capturedCtx); ok && claims != nil {
		t.Error("expected no claims in context when no Authorization header")
	}
}

func TestAuthenticate_GarbageToken(t *testing.T) {
	priv := generateTestKey(t)
	const kid = "k1"
	const issuer = "http://keycloak/realms/test"

	jwksSrv := newFakeJWKSServer(t, priv, kid)
	defer jwksSrv.Close()

	v := buildValidator(t, jwksSrv.URL, issuer)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer this.is.garbage")
	rec := httptest.NewRecorder()
	v.Authenticate(next).ServeHTTP(rec, req)

	// A present-but-invalid token must return 401 and never call next.
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if nextCalled {
		t.Error("next handler must not be called when token is invalid")
	}
}

func TestAuthenticate_ValidJWT_AdminRole(t *testing.T) {
	priv := generateTestKey(t)
	const kid = "k1"
	const issuer = "http://keycloak/realms/test"

	jwksSrv := newFakeJWKSServer(t, priv, kid)
	defer jwksSrv.Close()

	v := buildValidator(t, jwksSrv.URL, issuer)
	token := signJWT(t, priv, kid, issuer, []string{"default-roles-registry", "admin"})

	var capturedCtx context.Context
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	v.Authenticate(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	claims, ok := auth.ClaimsFromContext(capturedCtx)
	if !ok || claims == nil {
		t.Fatal("expected claims in context for valid admin JWT")
	}
	if !claims.IsAdmin() {
		t.Error("expected IsAdmin()=true for admin role JWT")
	}
	if !auth.IsAdminFromContext(capturedCtx) {
		t.Error("expected IsAdminFromContext=true for admin role JWT")
	}
}

func TestAuthenticate_ValidJWT_NonAdminRole(t *testing.T) {
	priv := generateTestKey(t)
	const kid = "k1"
	const issuer = "http://keycloak/realms/test"

	jwksSrv := newFakeJWKSServer(t, priv, kid)
	defer jwksSrv.Close()

	v := buildValidator(t, jwksSrv.URL, issuer)
	token := signJWT(t, priv, kid, issuer, []string{"viewer"})

	var capturedCtx context.Context
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	v.Authenticate(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	claims, ok := auth.ClaimsFromContext(capturedCtx)
	if !ok || claims == nil {
		t.Fatal("expected claims in context for valid non-admin JWT")
	}
	if claims.IsAdmin() {
		t.Error("expected IsAdmin()=false for non-admin JWT")
	}
	if auth.IsAdminFromContext(capturedCtx) {
		t.Error("expected IsAdminFromContext=false for non-admin JWT")
	}
}

func TestAuthenticate_ValidJWT_WrongIssuer(t *testing.T) {
	priv := generateTestKey(t)
	const kid = "k1"
	const serverIssuer = "http://keycloak/realms/test"
	const tokenIssuer = "http://evil/realms/hacker"

	jwksSrv := newFakeJWKSServer(t, priv, kid)
	defer jwksSrv.Close()

	v := buildValidator(t, jwksSrv.URL, serverIssuer)
	// Token signed with the right key but a different issuer.
	token := signJWT(t, priv, kid, tokenIssuer, []string{"admin"})

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	v.Authenticate(next).ServeHTTP(rec, req)

	// A token with the wrong issuer is invalid — must return 401.
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if nextCalled {
		t.Error("next handler must not be called for wrong-issuer JWT")
	}
}

// ---------------------------------------------------------------------------
// Expired / missing-exp JWT tests
// ---------------------------------------------------------------------------

// signJWTWithTimes signs a JWT with explicit iat and exp values.
// Pass exp = 0 to omit the exp field entirely.
func signJWTWithTimes(t *testing.T, priv *rsa.PrivateKey, kid, issuer string, roles []string, iat, exp int64) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss": issuer,
		"sub": "user-123",
		"iat": iat,
		"realm_access": map[string]interface{}{
			"roles": roles,
		},
	}
	if exp != 0 {
		claims["exp"] = exp
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid

	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("signing JWT: %v", err)
	}
	return signed
}

func TestAuthenticate_ExpiredJWT(t *testing.T) {
	priv := generateTestKey(t)
	const kid = "k1"
	const issuer = "http://keycloak/realms/test"

	jwksSrv := newFakeJWKSServer(t, priv, kid)
	defer jwksSrv.Close()

	v := buildValidator(t, jwksSrv.URL, issuer)

	// Token with exp 1 hour in the past.
	expiredAt := time.Now().Add(-time.Hour).Unix()
	token := signJWTWithTimes(t, priv, kid, issuer, []string{"admin"}, time.Now().Unix(), expiredAt)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	v.Authenticate(next).ServeHTTP(rec, req)

	// Expired tokens are invalid — must return 401.
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if nextCalled {
		t.Error("next handler must not be called for expired JWT")
	}
}

func TestAuthenticate_MissingExp(t *testing.T) {
	priv := generateTestKey(t)
	const kid = "k1"
	const issuer = "http://keycloak/realms/test"

	jwksSrv := newFakeJWKSServer(t, priv, kid)
	defer jwksSrv.Close()

	v := buildValidator(t, jwksSrv.URL, issuer)

	// Token without exp field (exp=0 means omit in signJWTWithTimes).
	token := signJWTWithTimes(t, priv, kid, issuer, []string{"admin"}, time.Now().Unix(), 0)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	v.Authenticate(next).ServeHTTP(rec, req)

	// golang-jwt rejects tokens without exp when WithExpirationRequired() is used.
	// Must return 401.
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if nextCalled {
		t.Error("next handler must not be called for JWT missing exp field")
	}
}

// ---------------------------------------------------------------------------
// bearerToken – tested indirectly via Authenticate
// ---------------------------------------------------------------------------

func TestBearerToken_ExtractedCorrectly(t *testing.T) {
	priv := generateTestKey(t)
	const kid = "k1"
	const issuer = "http://keycloak/realms/test"

	jwksSrv := newFakeJWKSServer(t, priv, kid)
	defer jwksSrv.Close()

	v := buildValidator(t, jwksSrv.URL, issuer)
	token := signJWT(t, priv, kid, issuer, []string{"admin"})

	tests := []struct {
		name          string
		headerValue   string
		expectClaims  bool
	}{
		{"correct Bearer prefix", "Bearer " + token, true},
		{"lowercase bearer", "bearer " + token, false}, // case-sensitive
		{"no prefix", token, false},
		{"empty header", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedCtx context.Context
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedCtx = r.Context()
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.headerValue != "" {
				req.Header.Set("Authorization", tt.headerValue)
			}
			rec := httptest.NewRecorder()
			v.Authenticate(next).ServeHTTP(rec, req)

			claims, ok := auth.ClaimsFromContext(capturedCtx)
			hasClaims := ok && claims != nil
			if hasClaims != tt.expectClaims {
				t.Errorf("hasClaims = %v, want %v", hasClaims, tt.expectClaims)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// IsAdminFromContext
// ---------------------------------------------------------------------------

func TestIsAdminFromContext_EmptyContext(t *testing.T) {
	if auth.IsAdminFromContext(context.Background()) {
		t.Error("IsAdminFromContext(empty) should return false")
	}
}

func TestIsAdminFromContext_TrueWhenAdminClaimsSet(t *testing.T) {
	claims := &auth.KeycloakClaims{RealmAccess: auth.RealmAccess{Roles: []string{"admin"}}}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	if !auth.IsAdminFromContext(ctx) {
		t.Error("IsAdminFromContext should return true for admin claims")
	}
}

func TestIsAdminFromContext_FalseWhenNonAdminClaimsSet(t *testing.T) {
	claims := &auth.KeycloakClaims{RealmAccess: auth.RealmAccess{Roles: []string{"viewer"}}}
	ctx := auth.ContextWithClaims(context.Background(), claims)
	if auth.IsAdminFromContext(ctx) {
		t.Error("IsAdminFromContext should return false for non-admin claims")
	}
}

// ---------------------------------------------------------------------------
// Audience validation
// ---------------------------------------------------------------------------

func TestAuthenticate_AudienceMatches(t *testing.T) {
	priv := generateTestKey(t)
	const kid = "k1"
	const issuer = "http://keycloak/realms/test"
	const audience = "ai-registry-server"

	jwksSrv := newFakeJWKSServer(t, priv, kid)
	defer jwksSrv.Close()

	v := buildValidatorWithAudience(t, jwksSrv.URL, issuer, audience)
	token := signJWTWithAudience(t, priv, kid, issuer, audience, []string{"admin"})

	var capturedCtx context.Context
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	v.Authenticate(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if claims, ok := auth.ClaimsFromContext(capturedCtx); !ok || claims == nil {
		t.Error("expected claims in context for matching-audience JWT")
	}
}

func TestAuthenticate_AudienceMismatch(t *testing.T) {
	priv := generateTestKey(t)
	const kid = "k1"
	const issuer = "http://keycloak/realms/test"

	jwksSrv := newFakeJWKSServer(t, priv, kid)
	defer jwksSrv.Close()

	// Validator expects this resource's audience; token was minted for a
	// different client on the same realm (the class of attack H1 addresses).
	v := buildValidatorWithAudience(t, jwksSrv.URL, issuer, "ai-registry-server")
	token := signJWTWithAudience(t, priv, kid, issuer, "some-other-client", []string{"admin"})

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	v.Authenticate(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if nextCalled {
		t.Error("next handler must not be called for wrong-audience JWT")
	}
}

func TestAuthenticate_AudienceMissing(t *testing.T) {
	priv := generateTestKey(t)
	const kid = "k1"
	const issuer = "http://keycloak/realms/test"

	jwksSrv := newFakeJWKSServer(t, priv, kid)
	defer jwksSrv.Close()

	// Validator requires an audience; token has no `aud` claim at all.
	v := buildValidatorWithAudience(t, jwksSrv.URL, issuer, "ai-registry-server")
	token := signJWTWithAudience(t, priv, kid, issuer, "", []string{"admin"})

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	v.Authenticate(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if nextCalled {
		t.Error("next handler must not be called for missing-audience JWT")
	}
}

// ---------------------------------------------------------------------------
// Config.JWKSEndpoint
// ---------------------------------------------------------------------------

func TestJWKSEndpoint_ReturnsExpectedURL(t *testing.T) {
	tests := []struct {
		name   string
		issuer string
		want   string
	}{
		{
			name:   "standard keycloak realm",
			issuer: "http://keycloak:8080/realms/registry",
			want:   "http://keycloak:8080/realms/registry/protocol/openid-connect/certs",
		},
		{
			name:   "production URL",
			issuer: "https://auth.example.com/realms/prod",
			want:   "https://auth.example.com/realms/prod/protocol/openid-connect/certs",
		},
		{
			name:   "empty issuer",
			issuer: "",
			want:   "/protocol/openid-connect/certs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := auth.Config{OIDCIssuer: tt.issuer}
			got := cfg.JWKSEndpoint()
			if got != tt.want {
				t.Errorf("JWKSEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

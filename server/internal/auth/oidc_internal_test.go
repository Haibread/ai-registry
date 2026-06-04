package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGeneratePKCE(t *testing.T) {
	v, c, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	sum := sha256.Sum256([]byte(v))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); c != want {
		t.Fatalf("challenge is not S256(verifier): got %q want %q", c, want)
	}
	v2, _, _ := GeneratePKCE()
	if v == v2 {
		t.Fatal("verifier must be random across calls")
	}
}

func TestAuthCodeURL(t *testing.T) {
	b := &OIDCBroker{
		cfg:   OIDCConfig{ClientID: "cid", RedirectURL: "https://app/cb"},
		disco: oidcDiscovery{AuthorizationEndpoint: "https://idp/auth"},
	}
	u, err := url.Parse(b.AuthCodeURL("st8", "nce", "chal"))
	if err != nil {
		t.Fatalf("parsing url: %v", err)
	}
	q := u.Query()
	for k, want := range map[string]string{
		"client_id":             "cid",
		"redirect_uri":          "https://app/cb",
		"response_type":         "code",
		"scope":                 "openid profile email",
		"state":                 "st8",
		"nonce":                 "nce",
		"code_challenge":        "chal",
		"code_challenge_method": "S256",
	} {
		if got := q.Get(k); got != want {
			t.Fatalf("authorize param %q = %q, want %q", k, got, want)
		}
	}
}

// newTestIdP stands up a minimal OIDC IdP (discovery + JWKS + token) signing
// id_tokens with a fresh RSA key. The returned *string lets a test set the
// nonce the token endpoint echoes.
func newTestIdP(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	nonce := new(string)
	var issuer string

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 issuer,
			"authorization_endpoint": issuer + "/auth",
			"token_endpoint":         issuer + "/token",
			"jwks_uri":               issuer + "/jwks",
			"end_session_endpoint":   issuer + "/logout",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		pub := key.Public().(*rsa.PublicKey)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{{
				"kty": "RSA",
				"kid": "k1",
				"use": "sig",
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			}},
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"iss":            issuer,
			"aud":            "cid",
			"exp":            time.Now().Add(time.Hour).Unix(),
			"iat":            time.Now().Unix(),
			"sub":            "sub-1",
			"email":          "alice@example.com",
			"email_verified": true,
			"nonce":          *nonce,
			"groups":         []string{"team-a"},
		})
		tok.Header["kid"] = "k1"
		signed, signErr := tok.SignedString(key)
		if signErr != nil {
			http.Error(w, signErr.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id_token":     signed,
			"access_token": "at",
			"token_type":   "Bearer",
		})
	})

	srv := httptest.NewServer(mux)
	issuer = srv.URL
	t.Cleanup(srv.Close)
	return srv, nonce
}

func TestOIDCBroker_Exchange(t *testing.T) {
	srv, nonce := newTestIdP(t)
	*nonce = "nonce-abc"
	ctx := context.Background()

	b, err := NewOIDCBroker(ctx, OIDCConfig{
		Issuer:       srv.URL,
		ClientID:     "cid",
		ClientSecret: "secret",
		RedirectURL:  "https://app/cb",
	})
	if err != nil {
		t.Fatalf("NewOIDCBroker: %v", err)
	}

	claims, idToken, err := b.Exchange(ctx, "code", "verifier", "nonce-abc")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if claims.Email != "alice@example.com" {
		t.Fatalf("unexpected email claim: %+v", claims)
	}
	if claims.Subject != "sub-1" {
		t.Fatalf("subject = %q, want sub-1", claims.Subject)
	}
	if len(claims.Groups) != 1 || claims.Groups[0] != "team-a" {
		t.Fatalf("groups not parsed: %+v", claims.Groups)
	}
	if idToken == "" {
		t.Fatal("expected raw id_token returned")
	}

	if _, _, err := b.Exchange(ctx, "code", "verifier", "wrong-nonce"); err == nil {
		t.Fatal("expected error on nonce mismatch")
	}
}

// TestOIDCBroker_Exchange_ConfigurableClaimPaths verifies the point-4 feature:
// groups and the admin role are read from operator-configured (and dotted) claim
// paths, not hard-coded ones. The IdP emits roles nested under
// resource_access.registry.roles and groups under team_groups; the broker is
// configured to read exactly those, and a custom admin role value.
func TestOIDCBroker_Exchange_ConfigurableClaimPaths(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	var issuer string
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 issuer,
			"authorization_endpoint": issuer + "/auth",
			"token_endpoint":         issuer + "/token",
			"jwks_uri":               issuer + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		pub := key.Public().(*rsa.PublicKey)
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"iss": issuer, "aud": "cid", "exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Unix(), "sub": "sub-1", "email": "alice@example.com",
			"email_verified": true, "nonce": "n1",
			"team_groups": []string{"x", "y"},
			"resource_access": map[string]any{
				"registry": map[string]any{"roles": []string{"superadmin", "reader"}},
			},
		})
		tok.Header["kid"] = "k1"
		signed, _ := tok.SignedString(key)
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": signed, "token_type": "Bearer"})
	})
	srv := httptest.NewServer(mux)
	issuer = srv.URL
	t.Cleanup(srv.Close)

	ctx := context.Background()
	b, err := NewOIDCBroker(ctx, OIDCConfig{
		Issuer: srv.URL, ClientID: "cid", ClientSecret: "secret", RedirectURL: "https://app/cb",
		GroupsClaim: "team_groups",
		RolesClaim:  "resource_access.registry.roles",
		AdminRole:   "superadmin",
	})
	if err != nil {
		t.Fatalf("NewOIDCBroker: %v", err)
	}

	id, _, err := b.Exchange(ctx, "code", "verifier", "n1")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if !id.IsAdmin {
		t.Error("the custom dotted roles path containing the custom admin role should set IsAdmin")
	}
	if len(id.Groups) != 2 || id.Groups[0] != "x" || id.Groups[1] != "y" {
		t.Errorf("groups from the custom claim path = %v, want [x y]", id.Groups)
	}

	// Negative control: the same token under the DEFAULT paths maps to no admin
	// and no groups — proving the mapping really followed the configured paths.
	def, err := NewOIDCBroker(ctx, OIDCConfig{
		Issuer: srv.URL, ClientID: "cid", ClientSecret: "secret", RedirectURL: "https://app/cb",
	})
	if err != nil {
		t.Fatalf("NewOIDCBroker(default): %v", err)
	}
	idDef, _, err := def.Exchange(ctx, "code", "verifier", "n1")
	if err != nil {
		t.Fatalf("Exchange(default): %v", err)
	}
	if idDef.IsAdmin {
		t.Error("default realm_access.roles path must NOT see the resource_access role")
	}
	if len(idDef.Groups) != 0 {
		t.Errorf("default groups path should find nothing, got %v", idDef.Groups)
	}
}

func TestRewriteHost(t *testing.T) {
	cases := []struct{ endpoint, base, want string }{
		{"https://pub.example/realms/r/token", "http://keycloak:8080", "http://keycloak:8080/realms/r/token"},
		{
			"https://pub.example/realms/r/protocol/openid-connect/certs",
			"http://keycloak:8080/realms/r",
			"http://keycloak:8080/realms/r/protocol/openid-connect/certs",
		},
	}
	for _, c := range cases {
		if got := rewriteHost(c.endpoint, c.base); got != c.want {
			t.Errorf("rewriteHost(%q, %q) = %q, want %q", c.endpoint, c.base, got, c.want)
		}
	}
	// A base with no host leaves the endpoint untouched.
	if got := rewriteHost("https://x/y", "not-a-url"); got != "https://x/y" {
		t.Errorf("expected unchanged endpoint on hostless base, got %q", got)
	}
}

// TestOIDCBroker_InternalURL covers the Docker split: the IdP is reachable only
// at the internal (httptest) host but advertises a public issuer/endpoints. The
// broker must fetch discovery + token + JWKS at the internal host while
// validating the public issuer and exposing the public authorize URL.
func TestOIDCBroker_InternalURL(t *testing.T) {
	const publicIssuer = "https://idp.public.example/realms/r"
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	// Handlers live under the realm path so the rewritten back-channel URLs
	// (which keep the advertised /realms/r/... path, only swapping the host)
	// resolve against this server.
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/r/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 publicIssuer,
			"authorization_endpoint": publicIssuer + "/auth",
			"token_endpoint":         publicIssuer + "/token",
			"jwks_uri":               publicIssuer + "/jwks",
		})
	})
	mux.HandleFunc("/realms/r/token", func(w http.ResponseWriter, _ *http.Request) {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"iss":   publicIssuer,
			"aud":   "cid",
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"sub":   "sub-1",
			"email": "alice@example.com",
			"nonce": "n1",
		})
		tok.Header["kid"] = "k1"
		signed, signErr := tok.SignedString(key)
		if signErr != nil {
			http.Error(w, signErr.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": signed, "token_type": "Bearer"})
	})
	mux.HandleFunc("/realms/r/jwks", func(w http.ResponseWriter, _ *http.Request) {
		pub := key.Public().(*rsa.PublicKey)
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	internalBase := srv.URL + "/realms/r"
	ctx := context.Background()
	b, err := NewOIDCBroker(ctx, OIDCConfig{
		Issuer:       publicIssuer,
		ClientID:     "cid",
		ClientSecret: "secret",
		RedirectURL:  "https://app/cb",
		InternalURL:  internalBase,
	})
	if err != nil {
		t.Fatalf("NewOIDCBroker: %v", err)
	}
	// Front-channel authorize URL keeps the public host.
	if got := b.AuthCodeURL("s", "n1", "c"); !strings.HasPrefix(got, publicIssuer+"/auth") {
		t.Fatalf("authorize URL = %q, want public host prefix", got)
	}
	// Token exchange succeeds only because token + JWKS were retargeted to the
	// internal host — the public host does not resolve in this test.
	claims, _, err := b.Exchange(ctx, "code", "verifier", "n1")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if claims.Email != "alice@example.com" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	// Fail-closed guard: when OIDC_ISSUER does not match what the IdP advertises,
	// construction errors instead of handing the browser unreachable URLs.
	if _, err := NewOIDCBroker(ctx, OIDCConfig{
		Issuer:       "https://wrong.example/realms/r",
		ClientID:     "cid",
		ClientSecret: "secret",
		InternalURL:  internalBase,
	}); err == nil {
		t.Fatal("expected issuer-mismatch error")
	}
}

func TestOIDCBroker_EndSessionURL(t *testing.T) {
	b := &OIDCBroker{
		cfg:   OIDCConfig{ClientID: "cid"},
		disco: oidcDiscovery{EndSessionEndpoint: "https://idp/logout"},
	}
	got := b.EndSessionURL("idhint", "https://app/")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := u.Query()
	if q.Get("id_token_hint") != "idhint" || q.Get("post_logout_redirect_uri") != "https://app/" {
		t.Fatalf("end-session params missing: %s", got)
	}
	if q.Get("client_id") != "cid" {
		t.Fatalf("end-session missing client_id: %s", got)
	}

	none := &OIDCBroker{}
	if none.EndSessionURL("x", "y") != "" {
		t.Fatal("expected empty end-session URL when IdP has none")
	}
}

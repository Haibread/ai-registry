package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// newServiceTokenBroker stands up a minimal IdP (discovery + JWKS) that signs
// with a fresh RSA key, builds a broker configured with audience, and returns
// the broker, its issuer, and a closure that mints RS256 access tokens with
// arbitrary claims so each case can shape iss / aud / roles / exp.
func newServiceTokenBroker(t *testing.T, audience string) (broker *OIDCBroker, issuer string, mint func(jwt.MapClaims) string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
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
	srv := httptest.NewServer(mux)
	issuer = srv.URL
	t.Cleanup(srv.Close)

	broker, err = NewOIDCBroker(context.Background(), OIDCConfig{
		Issuer:       srv.URL,
		ClientID:     "cid",
		ClientSecret: "secret",
		RedirectURL:  "https://app/cb",
		Audience:     audience,
	})
	if err != nil {
		t.Fatalf("NewOIDCBroker: %v", err)
	}

	mint = func(claims jwt.MapClaims) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = "k1"
		signed, signErr := tok.SignedString(key)
		if signErr != nil {
			t.Fatalf("signing token: %v", signErr)
		}
		return signed
	}
	return broker, issuer, mint
}

func TestVerifyServiceToken_DisabledWithoutAudience(t *testing.T) {
	b, issuer, mint := newServiceTokenBroker(t, "")
	tok := mint(jwt.MapClaims{
		"iss": issuer,
		"aud": "ai-registry",
		"sub": "svc",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	_, err := b.VerifyServiceToken(context.Background(), tok)
	if !errors.Is(err, ErrServiceTokensDisabled) {
		t.Fatalf("want ErrServiceTokensDisabled, got %v", err)
	}
}

func TestVerifyServiceToken_AdminServiceAccount(t *testing.T) {
	b, issuer, mint := newServiceTokenBroker(t, "ai-registry")
	// Shape of a Keycloak service-account access token: aud is an array, realm
	// roles carry "admin", groups list the SA's group memberships.
	tok := mint(jwt.MapClaims{
		"iss":          issuer,
		"aud":          []string{"account", "ai-registry"},
		"sub":          "service-account-tenant-operator",
		"email":        "op@platform.local",
		"exp":          time.Now().Add(time.Hour).Unix(),
		"iat":          time.Now().Unix(),
		"realm_access": map[string]any{"roles": []string{"admin", "uma_authorization"}},
		"groups":       []string{"platform-ops"},
	})
	id, err := b.VerifyServiceToken(context.Background(), tok)
	if err != nil {
		t.Fatalf("VerifyServiceToken: %v", err)
	}
	if id.Subject != "service-account-tenant-operator" {
		t.Fatalf("subject = %q", id.Subject)
	}
	if id.Email != "op@platform.local" {
		t.Fatalf("email = %q", id.Email)
	}
	if !id.IsAdmin {
		t.Fatal("realm admin role should map to IsAdmin")
	}
	if len(id.Groups) != 1 || id.Groups[0] != "platform-ops" {
		t.Fatalf("groups = %+v", id.Groups)
	}
}

func TestVerifyServiceToken_NonAdmin(t *testing.T) {
	b, issuer, mint := newServiceTokenBroker(t, "ai-registry")
	tok := mint(jwt.MapClaims{
		"iss":          issuer,
		"aud":          "ai-registry",
		"sub":          "svc",
		"exp":          time.Now().Add(time.Hour).Unix(),
		"realm_access": map[string]any{"roles": []string{"uma_authorization"}},
		"groups":       []string{"acme-editors"},
	})
	id, err := b.VerifyServiceToken(context.Background(), tok)
	if err != nil {
		t.Fatalf("VerifyServiceToken: %v", err)
	}
	if id.IsAdmin {
		t.Fatal("no admin role → IsAdmin must be false")
	}
	if len(id.Groups) != 1 || id.Groups[0] != "acme-editors" {
		t.Fatalf("groups = %+v", id.Groups)
	}
}

func TestVerifyServiceToken_Rejections(t *testing.T) {
	b, issuer, mint := newServiceTokenBroker(t, "ai-registry")
	cases := []struct {
		name   string
		claims jwt.MapClaims
	}{
		{
			name: "audience mismatch",
			claims: jwt.MapClaims{
				"iss": issuer, "aud": "some-other-client", "sub": "svc",
				"exp": time.Now().Add(time.Hour).Unix(),
			},
		},
		{
			name: "missing audience",
			claims: jwt.MapClaims{
				"iss": issuer, "sub": "svc",
				"exp": time.Now().Add(time.Hour).Unix(),
			},
		},
		{
			name: "issuer mismatch",
			claims: jwt.MapClaims{
				"iss": "https://evil.example", "aud": "ai-registry", "sub": "svc",
				"exp": time.Now().Add(time.Hour).Unix(),
			},
		},
		{
			name: "expired",
			claims: jwt.MapClaims{
				"iss": issuer, "aud": "ai-registry", "sub": "svc",
				"exp": time.Now().Add(-time.Hour).Unix(),
			},
		},
		{
			name: "no expiry",
			claims: jwt.MapClaims{
				"iss": issuer, "aud": "ai-registry", "sub": "svc",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tok := mint(tc.claims)
			if _, err := b.VerifyServiceToken(context.Background(), tok); err == nil {
				t.Fatalf("%s: expected rejection, got nil error", tc.name)
			}
		})
	}
}

func TestVerifyServiceToken_ForeignSignature(t *testing.T) {
	b, issuer, _ := newServiceTokenBroker(t, "ai-registry")
	// A token signed by a key the broker's JWKS does not publish must not verify,
	// even with the right issuer / audience / kid.
	foreign, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": issuer, "aud": "ai-registry", "sub": "svc",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = "k1"
	signed, err := tok.SignedString(foreign)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	if _, err := b.VerifyServiceToken(context.Background(), signed); err == nil {
		t.Fatal("a token signed by a foreign key must not verify")
	}
}

func TestVerifyServiceToken_NotAJWT(t *testing.T) {
	b, _, _ := newServiceTokenBroker(t, "ai-registry")
	if _, err := b.VerifyServiceToken(context.Background(), "not-a-jwt"); err == nil {
		t.Fatal("a non-JWT bearer must be rejected")
	}
}

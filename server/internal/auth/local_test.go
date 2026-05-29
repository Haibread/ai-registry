package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/haibread/ai-registry/internal/auth"
)

// testRSAKeyPEM generates an RSA key and returns it both as PKCS#8 PEM and as
// the live key (so tests can sign adversarial tokens with the same key).
func testRSAKeyPEM(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return string(pemBytes), key
}

func TestLocalIssuer_MintVerifyRoundTrip(t *testing.T) {
	pemKey, _ := testRSAKeyPEM(t)
	li, err := auth.NewLocalIssuer(pemKey, time.Hour)
	if err != nil {
		t.Fatalf("NewLocalIssuer: %v", err)
	}

	tok, err := li.Mint("user-ulid-123", "dev@example.com")
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	claims, err := li.Verify(tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Subject != "user-ulid-123" {
		t.Errorf("Subject = %q, want user-ulid-123", claims.Subject)
	}
	if claims.Email != "dev@example.com" {
		t.Errorf("Email = %q, want dev@example.com", claims.Email)
	}
	// Local tokens never carry realm admin.
	if claims.IsAdmin() {
		t.Error("local token must not carry realm admin")
	}
}

func TestLocalIssuer_PKCS1KeyAccepted(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if _, err := auth.NewLocalIssuer(string(pemBytes), time.Hour); err != nil {
		t.Fatalf("NewLocalIssuer with PKCS#1 key: %v", err)
	}
}

func TestNewLocalIssuer_Errors(t *testing.T) {
	if _, err := auth.NewLocalIssuer("", time.Hour); err == nil {
		t.Error("empty key should error")
	}
	if _, err := auth.NewLocalIssuer("not pem", time.Hour); err == nil {
		t.Error("non-PEM key should error")
	}
}

func TestLocalIssuer_RejectsWrongIssuer(t *testing.T) {
	pemKey, key := testRSAKeyPEM(t)
	li, err := auth.NewLocalIssuer(pemKey, time.Hour)
	if err != nil {
		t.Fatalf("NewLocalIssuer: %v", err)
	}

	// Sign a token with the SAME key but a foreign issuer — the signature is
	// valid, but issuer binding must reject it.
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": "some-other-issuer",
		"sub": "x",
		"aud": auth.LocalTokenAudience,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := li.Verify(signed); err == nil {
		t.Error("Verify accepted a token with the wrong issuer")
	}
}

func TestLocalIssuer_RejectsExpired(t *testing.T) {
	pemKey, key := testRSAKeyPEM(t)
	li, err := auth.NewLocalIssuer(pemKey, time.Hour)
	if err != nil {
		t.Fatalf("NewLocalIssuer: %v", err)
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": auth.LocalTokenIssuer,
		"sub": "x",
		"aud": auth.LocalTokenAudience,
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
		"exp": time.Now().Add(-time.Hour).Unix(),
	})
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := li.Verify(signed); err == nil {
		t.Error("Verify accepted an expired token")
	}
}

func TestLocalIssuer_JWKS(t *testing.T) {
	pemKey, _ := testRSAKeyPEM(t)
	li, err := auth.NewLocalIssuer(pemKey, time.Hour)
	if err != nil {
		t.Fatalf("NewLocalIssuer: %v", err)
	}
	jwks := li.JWKS()
	if len(jwks.Keys) != 1 {
		t.Fatalf("JWKS keys = %d, want 1", len(jwks.Keys))
	}
	k := jwks.Keys[0]
	if k.Kty != "RSA" || k.Use != "sig" {
		t.Errorf("JWK kty/use = %q/%q, want RSA/sig", k.Kty, k.Use)
	}
	if k.Kid != li.KID() || k.Kid == "" {
		t.Errorf("JWK kid = %q, want issuer KID %q", k.Kid, li.KID())
	}
	if k.N == "" || k.E == "" {
		t.Error("JWK modulus/exponent must be populated")
	}
}

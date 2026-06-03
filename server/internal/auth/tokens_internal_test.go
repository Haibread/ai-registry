package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewTokenAuthority_SeedIsDeterministic(t *testing.T) {
	const seed = "this-is-a-sufficiently-long-signing-seed-0123456789"

	a, gen, err := NewTokenAuthority("", seed, "iss", time.Minute)
	if err != nil {
		t.Fatalf("NewTokenAuthority(a): %v", err)
	}
	if gen {
		t.Fatal("a seed-derived key must not be reported as generated/ephemeral")
	}
	b, _, err := NewTokenAuthority("", seed, "iss", time.Minute)
	if err != nil {
		t.Fatalf("NewTokenAuthority(b): %v", err)
	}

	// Same seed → same key: a token minted by one authority verifies on the other.
	token, _, err := a.Mint(MintParams{UserID: "u1", AuthMethod: "local"})
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if _, err := b.Verify(token); err != nil {
		t.Fatalf("token from authority a must verify on b built from the same seed: %v", err)
	}
	if a.current.kid != b.current.kid {
		t.Fatalf("same seed should yield the same kid: %q vs %q", a.current.kid, b.current.kid)
	}
}

func TestNewTokenAuthority_DifferentSeedsDiverge(t *testing.T) {
	a, _, _ := NewTokenAuthority("", strings.Repeat("a", 40), "iss", time.Minute)
	b, _, _ := NewTokenAuthority("", strings.Repeat("b", 40), "iss", time.Minute)

	token, _, err := a.Mint(MintParams{UserID: "u1", AuthMethod: "local"})
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if _, err := b.Verify(token); err == nil {
		t.Fatal("a token signed under a different seed must not verify")
	}
}

func TestNewTokenAuthority_SeedTooShort(t *testing.T) {
	if _, _, err := NewTokenAuthority("", "too-short", "iss", time.Minute); err == nil {
		t.Fatal("a seed shorter than the minimum must be rejected")
	}
}

func TestNewTokenAuthority_PEMTakesPrecedenceOverSeed(t *testing.T) {
	// A valid seed plus an invalid PEM must fail on the PEM (PEM wins), proving
	// precedence rather than silently falling back to the seed.
	if _, _, err := NewTokenAuthority("not-a-pem", strings.Repeat("x", 40), "iss", time.Minute); err == nil {
		t.Fatal("an invalid PEM must error even when a seed is also provided")
	}
}

// ed25519PEM marshals a freshly generated Ed25519 private key to PKCS#8 PEM.
func ed25519PEM(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestNewTokenAuthority_PEMRoundTrip(t *testing.T) {
	keyPEM := ed25519PEM(t)
	a, gen, err := NewTokenAuthority(keyPEM, "", "iss", 0) // zero TTL → default
	if err != nil {
		t.Fatalf("NewTokenAuthority: %v", err)
	}
	if gen {
		t.Fatal("a provided PEM key must not be reported as generated")
	}
	if a.AccessTTL() != 15*time.Minute {
		t.Fatalf("AccessTTL = %v, want the 15m default for a zero TTL", a.AccessTTL())
	}
	token, expiresIn, err := a.Mint(MintParams{UserID: "u1", AuthMethod: "local"})
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if expiresIn != int((15 * time.Minute).Seconds()) {
		t.Errorf("expiresIn = %d, want 900", expiresIn)
	}
	if _, err := a.Verify(token); err != nil {
		t.Fatalf("a token minted with the PEM key must verify: %v", err)
	}
}

func TestTokenAuthority_JWKS(t *testing.T) {
	a := testAuthority(t)
	doc := a.JWKS()
	if len(doc.Keys) != 1 {
		t.Fatalf("JWKS should expose exactly one key, got %d", len(doc.Keys))
	}
	k := doc.Keys[0]
	if k.Kty != "OKP" || k.Crv != "Ed25519" || k.Use != "sig" || k.Alg != "EdDSA" {
		t.Fatalf("unexpected JWK header: %+v", k)
	}
	if k.Kid != a.current.kid {
		t.Errorf("JWK kid %q != current kid %q", k.Kid, a.current.kid)
	}
	raw, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		t.Fatalf("JWK x is not a 32-byte base64url Ed25519 key: len=%d err=%v", len(raw), err)
	}
}

// craftEdDSA signs a token with the authority's own private key but arbitrary
// claims, so a test can probe what Verify accepts/rejects beyond what Mint emits.
func craftEdDSA(a *TokenAuthority, claims jwt.RegisteredClaims) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, RegistryClaims{RegisteredClaims: claims})
	tok.Header["kid"] = a.current.kid
	s, err := tok.SignedString(a.current.priv)
	if err != nil {
		panic(err)
	}
	return s
}

func validClaims() jwt.RegisteredClaims {
	now := time.Now()
	return jwt.RegisteredClaims{
		Issuer:    "test-issuer",
		Audience:  jwt.ClaimStrings{TokenAudience},
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
}

// TestVerify_RejectsAlgorithmConfusion is the headline JWT-security test: a
// token whose header lies about the algorithm must be rejected. We forge two
// classic attacks — `alg: none`, and HS256 using the Ed25519 PUBLIC key as the
// HMAC secret (the public key is, by definition, public) — both stamped with the
// real kid. WithValidMethods(["EdDSA"]) must reject them before the key is even
// consulted.
func TestVerify_RejectsAlgorithmConfusion(t *testing.T) {
	a := testAuthority(t)

	none := jwt.NewWithClaims(jwt.SigningMethodNone, RegistryClaims{RegisteredClaims: validClaims()})
	none.Header["kid"] = a.current.kid
	noneStr, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("signing none token: %v", err)
	}
	if _, err := a.Verify(noneStr); err == nil {
		t.Fatal("a token with alg=none must be rejected")
	}

	hs := jwt.NewWithClaims(jwt.SigningMethodHS256, RegistryClaims{RegisteredClaims: validClaims()})
	hs.Header["kid"] = a.current.kid
	hsStr, err := hs.SignedString([]byte(a.current.pub)) // public key as HMAC secret
	if err != nil {
		t.Fatalf("signing HS256 token: %v", err)
	}
	if _, err := a.Verify(hsStr); err == nil {
		t.Fatal("an HS256 token forged with the public key must be rejected (alg confusion)")
	}
}

func TestVerify_EnforcesExpiryIssuerAudience(t *testing.T) {
	a := testAuthority(t)

	// Sanity: a correctly-crafted token verifies, proving craftEdDSA is sound.
	if _, err := a.Verify(craftEdDSA(a, validClaims())); err != nil {
		t.Fatalf("a valid crafted token should verify: %v", err)
	}

	expired := validClaims()
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	if _, err := a.Verify(craftEdDSA(a, expired)); err == nil {
		t.Fatal("an expired token must be rejected")
	}

	wrongIss := validClaims()
	wrongIss.Issuer = "evil-issuer"
	if _, err := a.Verify(craftEdDSA(a, wrongIss)); err == nil {
		t.Fatal("a token with the wrong issuer must be rejected")
	}

	wrongAud := validClaims()
	wrongAud.Audience = jwt.ClaimStrings{"some-other-service"}
	if _, err := a.Verify(craftEdDSA(a, wrongAud)); err == nil {
		t.Fatal("a token with the wrong audience must be rejected")
	}
}

func TestIsAdminFromContext(t *testing.T) {
	admin := ContextWithClaims(t.Context(), &KeycloakClaims{RealmAccess: RealmAccess{Roles: []string{"admin"}}})
	if !IsAdminFromContext(admin) {
		t.Fatal("expected admin context to report admin")
	}
	if IsAdminFromContext(t.Context()) {
		t.Fatal("a bare context must not report admin")
	}
}

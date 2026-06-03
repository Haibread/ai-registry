package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenAudience is the `aud` claim the registry stamps on its access tokens and
// requires on verification. The registry is the single token authority, so a
// constant audience is sufficient.
const TokenAudience = "ai-registry"

// RegistryClaims is the payload of a registry-issued access token. The token
// carries identity and the snapshotted OIDC inputs (claim groups + the global
// Server-Admin flag); per-publisher roles are NOT in the token — they are
// resolved live from role_grants on every write. Groups + SrvAdmin go stale at
// most one access-token TTL (the refresh re-reads them).
type RegistryClaims struct {
	jwt.RegisteredClaims
	Email      string   `json:"email,omitempty"`
	Groups     []string `json:"groups,omitempty"`
	SrvAdmin   bool     `json:"srvAdmin"`
	AuthMethod string   `json:"authMethod,omitempty"`
}

// signingKey bundles an Ed25519 key pair with its derived key id (kid).
type signingKey struct {
	kid  string
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

// TokenAuthority mints and verifies registry access tokens (Ed25519 / EdDSA
// JWTs). It signs with the current key and verifies against a set of public
// keys (current plus any retired-but-still-valid keys) addressed by kid, so a
// key can be rotated without instantly invalidating live tokens.
type TokenAuthority struct {
	issuer     string
	accessTTL  time.Duration
	current    signingKey
	verifyKeys map[string]ed25519.PublicKey
}

// MintParams describes the access token to issue for a caller.
type MintParams struct {
	UserID     string
	Email      string
	Groups     []string
	SrvAdmin   bool
	AuthMethod string // "oidc" | "local"
}

// minSigningSeedLen is the minimum length of a JWT_SIGNING_SEED. The seed is
// hashed to a 32-byte Ed25519 seed, so the derived key is only as unguessable as
// the input string — a short / low-entropy seed yields a forgeable signing key.
// Mirror the deployment's 12-char password floor, but require more: a signing
// key compromise forges every token, so demand at least 32 characters and
// document that it should be high-entropy (e.g. `openssl rand -hex 32`).
const minSigningSeedLen = 32

// NewTokenAuthority builds a TokenAuthority. The signing key is resolved with
// this precedence:
//
//   - pemKey (PEM-encoded PKCS#8 Ed25519 private key), when non-empty;
//   - else seed: an arbitrary high-entropy secret string, deterministically
//     hashed to the 32-byte Ed25519 seed (same seed → same key on every replica
//     and across restarts). Must be at least minSigningSeedLen characters;
//   - else an ephemeral key is generated (dev only) and `generated` is true so
//     the caller can warn.
//
// issuer is the `iss` the registry stamps and requires (the public base URL).
func NewTokenAuthority(pemKey, seed, issuer string, accessTTL time.Duration) (ta *TokenAuthority, generated bool, err error) {
	var priv ed25519.PrivateKey
	switch {
	case pemKey != "":
		if priv, err = parseEd25519PrivateKeyPEM(pemKey); err != nil {
			return nil, false, err
		}
	case seed != "":
		if len(seed) < minSigningSeedLen {
			return nil, false, fmt.Errorf("jwt signing seed: must be at least %d characters (high-entropy, e.g. `openssl rand -hex 32`)", minSigningSeedLen)
		}
		sum := sha256.Sum256([]byte(seed))
		priv = ed25519.NewKeyFromSeed(sum[:])
	default:
		if _, priv, err = ed25519.GenerateKey(rand.Reader); err != nil {
			return nil, false, fmt.Errorf("generating ephemeral signing key: %w", err)
		}
		generated = true
	}
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return nil, false, errors.New("jwt signing key: derived public key is not Ed25519")
	}
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	kid := keyID(pub)
	return &TokenAuthority{
		issuer:     issuer,
		accessTTL:  accessTTL,
		current:    signingKey{kid: kid, priv: priv, pub: pub},
		verifyKeys: map[string]ed25519.PublicKey{kid: pub},
	}, generated, nil
}

// AccessTTL is the lifetime stamped on minted access tokens.
func (a *TokenAuthority) AccessTTL() time.Duration { return a.accessTTL }

// Mint signs an access token for p and returns it with its lifetime in seconds.
func (a *TokenAuthority) Mint(p MintParams) (token string, expiresIn int, err error) {
	now := time.Now()
	jti, err := randomToken()
	if err != nil {
		return "", 0, err
	}
	claims := RegistryClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.issuer,
			Subject:   p.UserID,
			Audience:  jwt.ClaimStrings{TokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.accessTTL)),
			ID:        jti,
		},
		Email:      p.Email,
		Groups:     p.Groups,
		SrvAdmin:   p.SrvAdmin,
		AuthMethod: p.AuthMethod,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = a.current.kid
	signed, err := tok.SignedString(a.current.priv)
	if err != nil {
		return "", 0, fmt.Errorf("signing access token: %w", err)
	}
	return signed, int(a.accessTTL.Seconds()), nil
}

// Verify validates a token's signature (by kid), issuer, audience, and expiry,
// and returns its claims. It is pure crypto — no database access — so it sits on
// the hot read path.
func (a *TokenAuthority) Verify(tokenStr string) (*RegistryClaims, error) {
	claims := &RegistryClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		pub, ok := a.verifyKeys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown key id %q", kid)
		}
		return pub, nil
	},
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithIssuer(a.issuer),
		jwt.WithAudience(TokenAudience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("verifying access token: %w", err)
	}
	return claims, nil
}

// ── JWKS publication ─────────────────────────────────────────────────────────

// PublicJWK is one Ed25519 (OKP) public key in JWK form, served at
// /.well-known/jwks.json so other services can verify registry access tokens.
type PublicJWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
}

// JWKSDocument is the JSON Web Key Set served to verifiers.
type JWKSDocument struct {
	Keys []PublicJWK `json:"keys"`
}

// JWKS returns the public verification keys as a JWK set.
func (a *TokenAuthority) JWKS() JWKSDocument {
	keys := make([]PublicJWK, 0, len(a.verifyKeys))
	for kid, pub := range a.verifyKeys {
		keys = append(keys, PublicJWK{
			Kty: "OKP",
			Crv: "Ed25519",
			X:   base64.RawURLEncoding.EncodeToString(pub),
			Kid: kid,
			Use: "sig",
			Alg: "EdDSA",
		})
	}
	return JWKSDocument{Keys: keys}
}

// parseEd25519PrivateKeyPEM decodes a PEM-encoded PKCS#8 Ed25519 private key.
func parseEd25519PrivateKeyPEM(pemStr string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("jwt signing key: not valid PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jwt signing key: parsing PKCS#8: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("jwt signing key: not an Ed25519 key")
	}
	return priv, nil
}

// keyID derives a stable, short key id from a public key (first 16 chars of the
// base64url SHA-256 of the raw key bytes).
func keyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return base64.RawURLEncoding.EncodeToString(sum[:])[:16]
}

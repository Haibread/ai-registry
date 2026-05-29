package auth

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// LocalTokenIssuer is the `iss` value stamped on registry-signed local tokens.
// It is deliberately NOT a URL: it is a marker that distinguishes locally
// issued tokens from federated (OIDC) ones so the MCP wall (ADR 0006 §3) can
// reject local tokens on the OAuth-only protocol surface.
const LocalTokenIssuer = "ai-registry-local"

// LocalTokenAudience is the audience local tokens are minted for — the
// human/admin API. The MCP routes never accept this audience.
const LocalTokenAudience = "ai-registry-admin"

// ErrLocalIssuerNotConfigured is returned by LocalIssuer methods when local
// login is enabled but no signing key was supplied.
var ErrLocalIssuerNotConfigured = errors.New("local token issuer not configured")

// LocalIssuer signs and verifies the registry's own local-login tokens using
// an RSA key (RS256), and publishes the matching public key as a JWKS so the
// validator can self-verify exactly like the OIDC path (ADR 0006 decision:
// RS256 + JWKS). One process holds one key; rotating it invalidates
// outstanding local tokens (users re-login).
type LocalIssuer struct {
	privateKey *rsa.PrivateKey
	kid        string
	ttl        time.Duration
}

// NewLocalIssuer parses a PEM-encoded RSA private key (PKCS#1 or PKCS#8) and
// returns an issuer that mints tokens valid for ttl (defaulting to 1h when
// non-positive). The kid is derived deterministically from the public key so
// it is stable across restarts that reuse the same key.
func NewLocalIssuer(pemKey string, ttl time.Duration) (*LocalIssuer, error) {
	if pemKey == "" {
		return nil, ErrLocalIssuerNotConfigured
	}
	key, err := parseRSAPrivateKeyPEM(pemKey)
	if err != nil {
		return nil, err
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &LocalIssuer{
		privateKey: key,
		kid:        publicKeyKID(&key.PublicKey),
		ttl:        ttl,
	}, nil
}

// Mint signs a local access token for the resolved principal. The token
// carries only the principal id (sub) and email — Server Admin is NOT embedded
// (it is read live from users.is_server_admin so a revoked flag takes effect
// immediately), and groups come from the DB, not the token.
func (li *LocalIssuer) Mint(userID, email string) (string, error) {
	if li == nil {
		return "", ErrLocalIssuerNotConfigured
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   LocalTokenIssuer,
		"sub":   userID,
		"email": email,
		"aud":   LocalTokenAudience,
		"iat":   now.Unix(),
		"exp":   now.Add(li.ttl).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = li.kid
	signed, err := tok.SignedString(li.privateKey)
	if err != nil {
		return "", fmt.Errorf("signing local token: %w", err)
	}
	return signed, nil
}

// Verify validates a local token's signature, issuer, audience, and expiry,
// and returns it as KeycloakClaims (Subject = the users.id, Email set;
// RealmAccess and Groups intentionally empty — local tokens never carry realm
// admin or claim groups). Use only after the caller has confirmed the token's
// `iss` is LocalTokenIssuer.
func (li *LocalIssuer) Verify(tokenString string) (*KeycloakClaims, error) {
	if li == nil {
		return nil, ErrLocalIssuerNotConfigured
	}
	claims := &KeycloakClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return &li.privateKey.PublicKey, nil
	},
		jwt.WithIssuedAt(),
		jwt.WithIssuer(LocalTokenIssuer),
		jwt.WithAudience(LocalTokenAudience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// JWKS returns the issuer's public key as a single-entry JSON Web Key Set,
// served at the registry's JWKS endpoint for self-verification.
func (li *LocalIssuer) JWKS() jwksResponse {
	if li == nil {
		return jwksResponse{Keys: []jwksKey{}}
	}
	pub := &li.privateKey.PublicKey
	return jwksResponse{Keys: []jwksKey{{
		Kty: "RSA",
		Kid: li.kid,
		Use: "sig",
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big0(pub.E)),
	}}}
}

// KID exposes the issuer's key id (for tests / diagnostics).
func (li *LocalIssuer) KID() string { return li.kid }

// TTL is the lifetime minted tokens are valid for (surfaced as expires_in).
func (li *LocalIssuer) TTL() time.Duration {
	if li == nil {
		return 0
	}
	return li.ttl
}

// JWKSJSON returns the issuer's public JWKS as a value ready for JSON
// encoding. Empty (no keys) when the issuer is not configured.
func (li *LocalIssuer) JWKSJSON() any { return li.JWKS() }

func parseRSAPrivateKeyPEM(pemKey string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, fmt.Errorf("AUTH_LOCAL_SIGNING_KEY is not valid PEM")
	}
	// Try PKCS#8 first, then fall back to PKCS#1.
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("AUTH_LOCAL_SIGNING_KEY is not an RSA key")
		}
		return rsaKey, nil
	}
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("AUTH_LOCAL_SIGNING_KEY is not a usable RSA private key (want PKCS#1 or PKCS#8 PEM): %w", err)
	}
	return rsaKey, nil
}

// publicKeyKID derives a stable key id from the public key's DER encoding.
func publicKeyKID(pub *rsa.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		// Fall back to the modulus bytes; collisions are not a concern for a
		// single-key issuer.
		der = pub.N.Bytes()
	}
	sum := sha256.Sum256(der)
	return base64.RawURLEncoding.EncodeToString(sum[:])[:16]
}

// big0 encodes the RSA public exponent as the minimal big-endian byte slice a
// JWK expects (e.g. 65537 -> {0x01,0x00,0x01}).
func big0(e int) []byte {
	if e <= 0 {
		return []byte{0}
	}
	var b []byte
	for e > 0 {
		b = append([]byte{byte(e & 0xff)}, b...)
		e >>= 8
	}
	return b
}

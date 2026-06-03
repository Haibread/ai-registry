package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// randomToken returns a 256-bit URL-safe random string. It backs every opaque
// secret the auth layer mints: refresh tokens, OIDC state/nonce, and one-time
// handoff codes.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// RandomToken returns a 256-bit URL-safe random string, used by the OIDC broker
// handlers for the login-transaction state and nonce.
func RandomToken() (string, error) {
	return randomToken()
}

// HashToken returns the hex SHA-256 of an opaque token, for callers (the OIDC
// handlers) that mint a token here and must persist or look it up by its hash —
// the login-transaction state and the one-time handoff code.
func HashToken(token string) string {
	return hashToken(token)
}

// hashToken returns the hex SHA-256 of an opaque token — the value persisted and
// looked up, never the raw token. Refresh tokens, OIDC state, and handoff codes
// are all stored only as this hash, so a DB leak yields nothing usable.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

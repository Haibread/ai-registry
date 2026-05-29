package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Local-account passwords are hashed with argon2id (ADR 0006 §2). This is the
// project's first stored credential — handle per the secrets conventions.
//
// The encoded form is the PHC string the rest of the world recognises:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<base64 salt>$<base64 hash>
//
// storing the parameters inline lets us tune them later without invalidating
// existing hashes — VerifyPassword reads the cost from the stored string.

// argon2idParams are the default cost parameters for newly-hashed passwords.
// memoryKiB=64 MiB, time=3 passes, parallelism=2 follow OWASP's argon2id
// guidance; saltLen/keyLen are 16/32 bytes.
type argon2idParams struct {
	memoryKiB   uint32
	iterations  uint32
	parallelism uint8
	saltLen     uint32
	keyLen      uint32
}

var defaultArgon2idParams = argon2idParams{
	memoryKiB:   64 * 1024,
	iterations:  3,
	parallelism: 2,
	saltLen:     16,
	keyLen:      32,
}

// ErrInvalidPasswordHash is returned when a stored hash string cannot be
// parsed (corrupt column, wrong algorithm, truncated value).
var ErrInvalidPasswordHash = errors.New("invalid password hash")

// HashPassword derives an argon2id hash of password and returns it in PHC
// string form, ready to store in users.password_hash.
func HashPassword(password string) (string, error) {
	return hashPasswordWith(password, defaultArgon2idParams)
}

func hashPasswordWith(password string, p argon2idParams) (string, error) {
	salt := make([]byte, p.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, p.iterations, p.memoryKiB, p.parallelism, p.keyLen)

	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memoryKiB, p.iterations, p.parallelism,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches the stored argon2id hash.
// The comparison is constant-time. A malformed encoded hash returns
// ErrInvalidPasswordHash; a well-formed hash that simply does not match
// returns (false, nil).
func VerifyPassword(password, encoded string) (bool, error) {
	p, salt, want, err := decodeArgon2idHash(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, p.iterations, p.memoryKiB, p.parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func decodeArgon2idHash(encoded string) (argon2idParams, []byte, []byte, error) {
	var p argon2idParams
	parts := strings.Split(encoded, "$")
	// "" / "argon2id" / "v=19" / "m=..,t=..,p=.." / salt / hash
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, nil, nil, ErrInvalidPasswordHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, nil, nil, ErrInvalidPasswordHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memoryKiB, &p.iterations, &p.parallelism); err != nil {
		return p, nil, nil, ErrInvalidPasswordHash
	}

	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, ErrInvalidPasswordHash
	}
	hash, err := b64.DecodeString(parts[5])
	if err != nil {
		return p, nil, nil, ErrInvalidPasswordHash
	}
	return p, salt, hash, nil
}

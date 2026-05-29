package auth_test

import (
	"strings"
	"testing"

	"github.com/haibread/ai-registry/internal/auth"
)

func TestHashPassword_RoundTrip(t *testing.T) {
	const pw = "correct horse battery staple"
	hash, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash %q is not in argon2id PHC form", hash)
	}

	ok, err := auth.VerifyPassword(pw, hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("VerifyPassword returned false for the correct password")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, err := auth.HashPassword("the-right-one")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := auth.VerifyPassword("the-wrong-one", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Error("VerifyPassword returned true for a wrong password")
	}
}

func TestHashPassword_SaltedUnique(t *testing.T) {
	// The same password must hash to different strings (random per-hash salt),
	// and both must still verify.
	a, err := auth.HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword a: %v", err)
	}
	b, err := auth.HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword b: %v", err)
	}
	if a == b {
		t.Error("identical passwords produced identical hashes — salt is not random")
	}
	for _, h := range []string{a, b} {
		ok, err := auth.VerifyPassword("same", h)
		if err != nil || !ok {
			t.Errorf("VerifyPassword(%q) = (%v, %v), want (true, nil)", h, ok, err)
		}
	}
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	for _, bad := range []string{
		"",
		"not-a-hash",
		"$argon2id$v=19$bad-params$c2FsdA$aGFzaA",
		"$bcrypt$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		"$argon2id$v=1$m=65536,t=3,p=2$c2FsdA$aGFzaA", // wrong version
	} {
		ok, err := auth.VerifyPassword("whatever", bad)
		if err == nil {
			t.Errorf("VerifyPassword(%q) error = nil, want ErrInvalidPasswordHash", bad)
		}
		if ok {
			t.Errorf("VerifyPassword(%q) = true for a malformed hash", bad)
		}
	}
}

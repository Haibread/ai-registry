package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/haibread/ai-registry/internal/store"
)

func TestCreateUser_AndLookups(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	created, err := sharedDB.CreateUser(ctx, store.CreateUserParams{
		Email:        "Alice@Example.COM", // mixed case — must be normalised
		DisplayName:  "Alice",
		Subject:      "oidc-sub-123",
		PasswordHash: "$argon2id$stub",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.Email != "alice@example.com" {
		t.Errorf("email not normalised: got %q", created.Email)
	}
	if !created.HasPassword {
		t.Error("HasPassword should be true when a hash was supplied")
	}

	// Lookup by id, subject, and (case-insensitive) email all resolve the row.
	byID, err := sharedDB.GetUserByID(ctx, created.ID)
	if err != nil || byID.ID != created.ID {
		t.Fatalf("GetUserByID: %v (%+v)", err, byID)
	}
	bySub, err := sharedDB.GetUserBySubject(ctx, "oidc-sub-123")
	if err != nil || bySub.ID != created.ID {
		t.Fatalf("GetUserBySubject: %v", err)
	}
	byEmail, err := sharedDB.GetUserByEmail(ctx, "ALICE@example.com")
	if err != nil || byEmail.ID != created.ID {
		t.Fatalf("GetUserByEmail (case-insensitive): %v", err)
	}
}

func TestCreateUser_DuplicateEmailConflict(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	if _, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "dup@x.test"}); err != nil {
		t.Fatalf("first CreateUser: %v", err)
	}
	if _, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "DUP@x.test"}); !errors.Is(err, store.ErrConflict) {
		t.Errorf("duplicate email err = %v, want ErrConflict", err)
	}
}

func TestCredentialsByEmail(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	if _, err := sharedDB.CreateUser(ctx, store.CreateUserParams{
		Email:        "login@x.test",
		PasswordHash: "$argon2id$stored",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	creds, err := sharedDB.CredentialsByEmail(ctx, "login@x.test")
	if err != nil {
		t.Fatalf("CredentialsByEmail: %v", err)
	}
	if creds.PasswordHash != "$argon2id$stored" {
		t.Errorf("hash = %q, want stored value", creds.PasswordHash)
	}
	if creds.Disabled {
		t.Error("new user should not be disabled")
	}

	if _, err := sharedDB.CredentialsByEmail(ctx, "ghost@x.test"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing user err = %v, want ErrNotFound", err)
	}
}

// TestBindSubject_BindOnce verifies the bind-once invariant: an OIDC subject
// binds onto a row with no subject, but a later attempt to rebind a different
// subject is refused (the account-takeover guard).
func TestBindSubject_BindOnce(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Invited user: no subject, no password.
	u, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "invitee@x.test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Subject != "" {
		t.Fatalf("invited user should have no subject, got %q", u.Subject)
	}

	if err := sharedDB.BindSubject(ctx, u.ID, "sub-first"); err != nil {
		t.Fatalf("first BindSubject: %v", err)
	}
	bound, _ := sharedDB.GetUserByID(ctx, u.ID)
	if bound.Subject != "sub-first" {
		t.Errorf("subject = %q, want sub-first", bound.Subject)
	}

	// Rebinding a different subject must fail (bind-once).
	if err := sharedDB.BindSubject(ctx, u.ID, "sub-second"); !errors.Is(err, store.ErrConflict) {
		t.Errorf("rebind err = %v, want ErrConflict (bind-once)", err)
	}
}

func TestSetPasswordHash(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	u, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "nopass@x.test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.HasPassword {
		t.Fatal("user created without a hash should not have a password")
	}

	if err := sharedDB.SetPasswordHash(ctx, u.ID, "$argon2id$new"); err != nil {
		t.Fatalf("SetPasswordHash: %v", err)
	}
	creds, err := sharedDB.CredentialsByEmail(ctx, "nopass@x.test")
	if err != nil {
		t.Fatalf("CredentialsByEmail: %v", err)
	}
	if creds.PasswordHash != "$argon2id$new" {
		t.Errorf("hash = %q, want the value just set", creds.PasswordHash)
	}

	if err := sharedDB.SetPasswordHash(ctx, "nonexistent", "x"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SetPasswordHash on missing user err = %v, want ErrNotFound", err)
	}
}

func TestUpdateUser_PartialPatch(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	u, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "patch@x.test", DisplayName: "Before"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	disabled := true
	updated, err := sharedDB.UpdateUser(ctx, u.ID, store.UpdateUserParams{Disabled: &disabled})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if !updated.Disabled {
		t.Error("Disabled should be true after patch")
	}
	// DisplayName was not in the patch — must be untouched.
	if updated.DisplayName != "Before" {
		t.Errorf("DisplayName = %q, want unchanged 'Before'", updated.DisplayName)
	}
}

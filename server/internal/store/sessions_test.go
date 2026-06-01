package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/store"
)

func createSessionUser(t *testing.T, email string) string {
	t.Helper()
	u, err := sharedDB.CreateUser(context.Background(), store.CreateUserParams{Email: email})
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}
	return u.ID
}

func mustSession(t *testing.T, userID, hash, method string, exp time.Time) *store.Session {
	t.Helper()
	s, err := sharedDB.CreateSession(context.Background(), store.CreateSessionParams{
		UserID:     userID,
		TokenHash:  hash,
		AuthMethod: method,
		ExpiresAt:  exp,
	})
	if err != nil {
		t.Fatalf("CreateSession(%s): %v", hash, err)
	}
	return s
}

func TestSession_CreateAndLookup(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	uid := createSessionUser(t, "alice@example.com")

	s := mustSession(t, uid, "hash-1", "local", time.Now().Add(time.Hour))
	if s.ID == "" || s.UserID != uid || s.AuthMethod != "local" {
		t.Fatalf("unexpected session: %+v", s)
	}

	got, err := sharedDB.ActiveSessionByTokenHash(ctx, "hash-1")
	if err != nil {
		t.Fatalf("ActiveSessionByTokenHash: %v", err)
	}
	if got.ID != s.ID || got.UserID != uid {
		t.Fatalf("lookup mismatch: %+v", got)
	}
	if got.RevokedAt != nil {
		t.Fatalf("new session should not be revoked: %+v", got)
	}
}

func TestSession_SnapshotsClaims(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	uid := createSessionUser(t, "bob@example.com")

	if _, err := sharedDB.CreateSession(ctx, store.CreateSessionParams{
		UserID:      uid,
		TokenHash:   "hash-2",
		AuthMethod:  "oidc",
		ClaimGroups: []string{"team-a", "team-b"},
		ClaimAdmin:  true,
		IDToken:     "raw.id.token",
		ExpiresAt:   time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := sharedDB.ActiveSessionByTokenHash(ctx, "hash-2")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !got.ClaimAdmin || len(got.ClaimGroups) != 2 || got.ClaimGroups[0] != "team-a" {
		t.Fatalf("claims not snapshotted: %+v", got)
	}
	if got.IDToken != "raw.id.token" {
		t.Fatalf("id_token not persisted for OIDC session: %q", got.IDToken)
	}
}

func TestSession_LocalHasNoIDToken(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	uid := createSessionUser(t, "carol@example.com")

	if _, err := sharedDB.CreateSession(ctx, store.CreateSessionParams{
		UserID:     uid,
		TokenHash:  "hash-local",
		AuthMethod: "local",
		ExpiresAt:  time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	got, err := sharedDB.ActiveSessionByTokenHash(ctx, "hash-local")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.IDToken != "" {
		t.Fatalf("local session should have no id_token, got %q", got.IDToken)
	}
}

func TestSession_ExpiredNotReturned(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	uid := createSessionUser(t, "carol@example.com")
	mustSession(t, uid, "hash-3", "local", time.Now().Add(-time.Minute))

	if _, err := sharedDB.ActiveSessionByTokenHash(ctx, "hash-3"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for expired session, got %v", err)
	}
}

func TestSession_Revoke(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	uid := createSessionUser(t, "dave@example.com")
	mustSession(t, uid, "hash-4", "local", time.Now().Add(time.Hour))

	if err := sharedDB.RevokeSession(ctx, "hash-4"); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	if _, err := sharedDB.ActiveSessionByTokenHash(ctx, "hash-4"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after revoke, got %v", err)
	}
	if err := sharedDB.RevokeSession(ctx, "hash-4"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second revoke, got %v", err)
	}
}

func TestSession_DeleteExpired(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	uid := createSessionUser(t, "erin@example.com")
	mustSession(t, uid, "live", "local", time.Now().Add(time.Hour))
	mustSession(t, uid, "old", "local", time.Now().Add(-time.Hour))
	mustSession(t, uid, "rev", "local", time.Now().Add(time.Hour))
	if err := sharedDB.RevokeSession(ctx, "rev"); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	n, err := sharedDB.DeleteExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 deleted (expired + revoked), got %d", n)
	}
	if _, err := sharedDB.ActiveSessionByTokenHash(ctx, "live"); err != nil {
		t.Fatalf("live session should remain: %v", err)
	}
}

func TestSession_CascadesOnUserDelete(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	uid := createSessionUser(t, "frank@example.com")
	mustSession(t, uid, "hash-5", "local", time.Now().Add(time.Hour))

	if err := sharedDB.DeleteUser(ctx, uid); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := sharedDB.ActiveSessionByTokenHash(ctx, "hash-5"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("session should be gone after user delete, got %v", err)
	}
}

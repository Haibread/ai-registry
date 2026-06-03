package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/store"
)

func newUser(t *testing.T, ctx context.Context, email string) *store.User {
	t.Helper()
	u, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: email})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u
}

func TestRefreshToken_CreateAndRotate(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	u := newUser(t, ctx, "rt@x.test")

	_, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "hash-1", AuthMethod: "local",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}

	row, err := sharedDB.RotateRefreshToken(ctx, "hash-1", "hash-2", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("RotateRefreshToken: %v", err)
	}
	if row.UserID != u.ID || row.AuthMethod != "local" {
		t.Fatalf("rotated row carried wrong fields: %+v", row)
	}
	if row.RotatedFrom == nil {
		t.Error("successor should record rotated_from")
	}
}

func TestRefreshToken_DuplicateHashConflict(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	u := newUser(t, ctx, "dup@x.test")

	p := store.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "dup-hash", AuthMethod: "local", ExpiresAt: time.Now().Add(time.Hour),
	}
	if _, err := sharedDB.CreateRefreshToken(ctx, p); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := sharedDB.CreateRefreshToken(ctx, p); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate hash err = %v, want ErrConflict", err)
	}
}

func TestRefreshToken_ReuseRevokesLineage(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	u := newUser(t, ctx, "reuse@x.test")

	if _, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "h1", AuthMethod: "oidc", ClaimGroups: []string{"g1"},
		ClaimAdmin: true, IDToken: "idtok", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}

	// First rotation succeeds and carries the snapshot forward.
	row, err := sharedDB.RotateRefreshToken(ctx, "h1", "h2", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("first rotate: %v", err)
	}
	if !row.ClaimAdmin || len(row.ClaimGroups) != 1 || row.IDToken != "idtok" {
		t.Fatalf("snapshot not carried forward: %+v", row)
	}

	// Replaying the now-rotated h1 is reuse → ErrRefreshReuse + lineage revoked.
	if _, err := sharedDB.RotateRefreshToken(ctx, "h1", "h3", time.Now().Add(time.Hour)); !errors.Is(err, store.ErrRefreshReuse) {
		t.Fatalf("reuse rotate err = %v, want ErrRefreshReuse", err)
	}
	// The successor h2 must now be revoked too (whole lineage killed).
	if _, err := sharedDB.RotateRefreshToken(ctx, "h2", "h4", time.Now().Add(time.Hour)); !errors.Is(err, store.ErrRefreshReuse) {
		t.Fatalf("h2 after lineage revoke err = %v, want ErrRefreshReuse", err)
	}
}

func TestRefreshToken_RotateUnknownAndExpired(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	u := newUser(t, ctx, "exp@x.test")

	if _, err := sharedDB.RotateRefreshToken(ctx, "nope", "x", time.Now().Add(time.Hour)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown rotate err = %v, want ErrNotFound", err)
	}

	if _, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "old", AuthMethod: "local",
		ExpiresAt: time.Now().Add(-time.Minute), // already expired
	}); err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	if _, err := sharedDB.RotateRefreshToken(ctx, "old", "new", time.Now().Add(time.Hour)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired rotate err = %v, want ErrNotFound", err)
	}
}

func TestRefreshToken_RevokeAndRevokeAll(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	u := newUser(t, ctx, "rev@x.test")

	if _, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "rh1", AuthMethod: "oidc", IDToken: "idt",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	row, err := sharedDB.RevokeRefreshToken(ctx, "rh1")
	if err != nil {
		t.Fatalf("RevokeRefreshToken: %v", err)
	}
	if row.IDToken != "idt" {
		t.Errorf("revoke should return the row (for the id_token hint), got %+v", row)
	}
	// Second revoke of the same token is a no-op → ErrNotFound.
	if _, err := sharedDB.RevokeRefreshToken(ctx, "rh1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("double revoke err = %v, want ErrNotFound", err)
	}

	if _, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "rh2", AuthMethod: "local", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	n, err := sharedDB.RevokeAllRefreshTokensForUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("RevokeAllRefreshTokensForUser: %v", err)
	}
	if n != 1 { // rh1 already revoked; only rh2 remained live
		t.Errorf("revoked count = %d, want 1", n)
	}
}

func TestOIDCAuthRequest_ConsumeOnce(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	if err := sharedDB.CreateOIDCAuthRequest(ctx, store.CreateOIDCAuthRequestParams{
		StateHash: "st-1", Nonce: "n1", CodeVerifier: "v1", ExpiresAt: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatalf("CreateOIDCAuthRequest: %v", err)
	}
	nonce, verifier, err := sharedDB.ConsumeOIDCAuthRequest(ctx, "st-1")
	if err != nil || nonce != "n1" || verifier != "v1" {
		t.Fatalf("Consume: nonce=%q verifier=%q err=%v", nonce, verifier, err)
	}
	// Single use.
	if _, _, err := sharedDB.ConsumeOIDCAuthRequest(ctx, "st-1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second consume err = %v, want ErrNotFound", err)
	}

	// Expired requests are not consumable.
	if err := sharedDB.CreateOIDCAuthRequest(ctx, store.CreateOIDCAuthRequestParams{
		StateHash: "st-2", Nonce: "n", CodeVerifier: "v", ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("CreateOIDCAuthRequest: %v", err)
	}
	if _, _, err := sharedDB.ConsumeOIDCAuthRequest(ctx, "st-2"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired consume err = %v, want ErrNotFound", err)
	}
}

func TestRefreshToken_DeleteExpired(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	u := newUser(t, ctx, "sweep@x.test")

	// One live, one expired, one revoked.
	if _, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "live", AuthMethod: "local", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create live: %v", err)
	}
	if _, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "exp", AuthMethod: "local", ExpiresAt: time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("create expired: %v", err)
	}
	if _, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "rev", AuthMethod: "local", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create to-revoke: %v", err)
	}
	if _, err := sharedDB.RevokeRefreshToken(ctx, "rev"); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	n, err := sharedDB.DeleteExpiredRefreshTokens(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredRefreshTokens: %v", err)
	}
	if n != 2 { // expired + revoked, never the live one
		t.Errorf("deleted %d, want 2 (expired + revoked)", n)
	}
}

func TestOIDCAuthRequest_DeleteExpired(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	if err := sharedDB.CreateOIDCAuthRequest(ctx, store.CreateOIDCAuthRequestParams{
		StateHash: "s-live", Nonce: "n", CodeVerifier: "v", ExpiresAt: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatalf("create live: %v", err)
	}
	if err := sharedDB.CreateOIDCAuthRequest(ctx, store.CreateOIDCAuthRequestParams{
		StateHash: "s-exp", Nonce: "n", CodeVerifier: "v", ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("create expired: %v", err)
	}
	n, err := sharedDB.DeleteExpiredOIDCAuthRequests(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredOIDCAuthRequests: %v", err)
	}
	if n != 1 {
		t.Errorf("deleted %d, want 1", n)
	}
}

func TestHandoffCode_DeleteExpired(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	if err := sharedDB.CreateHandoffCode(ctx, store.CreateHandoffCodeParams{
		CodeHash: "live", AccessToken: "a", RefreshToken: "r", ExpiresIn: 900, ExpiresAt: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatalf("create live: %v", err)
	}
	if err := sharedDB.CreateHandoffCode(ctx, store.CreateHandoffCodeParams{
		CodeHash: "consumed", AccessToken: "a", RefreshToken: "r", ExpiresIn: 900, ExpiresAt: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatalf("create consumed: %v", err)
	}
	if _, _, _, err := sharedDB.ConsumeHandoffCode(ctx, "consumed"); err != nil {
		t.Fatalf("consume: %v", err)
	}
	n, err := sharedDB.DeleteExpiredHandoffCodes(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredHandoffCodes: %v", err)
	}
	if n != 1 { // the consumed one; the live unconsumed stays
		t.Errorf("deleted %d, want 1 (consumed)", n)
	}
}

func TestHandoffCode_ConsumeOnce(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	if err := sharedDB.CreateHandoffCode(ctx, store.CreateHandoffCodeParams{
		CodeHash: "c1", AccessToken: "acc", RefreshToken: "ref", ExpiresIn: 900,
		ExpiresAt: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatalf("CreateHandoffCode: %v", err)
	}
	acc, ref, exp, err := sharedDB.ConsumeHandoffCode(ctx, "c1")
	if err != nil || acc != "acc" || ref != "ref" || exp != 900 {
		t.Fatalf("Consume: acc=%q ref=%q exp=%d err=%v", acc, ref, exp, err)
	}
	if _, _, _, err := sharedDB.ConsumeHandoffCode(ctx, "c1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second consume err = %v, want ErrNotFound", err)
	}
}

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

	row, err := sharedDB.RotateRefreshToken(ctx, "hash-1", "hash-2")
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

func TestRefreshToken_RotationKeepsLineageExpiry(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	u := newUser(t, ctx, "lineage@x.test")

	expiresAt := time.Now().Add(time.Second).Truncate(time.Microsecond)
	if _, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "l1", AuthMethod: "oidc", ClaimAdmin: true,
		ExpiresAt: expiresAt,
	}); err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}

	row, err := sharedDB.RotateRefreshToken(ctx, "l1", "l2")
	if err != nil {
		t.Fatalf("RotateRefreshToken: %v", err)
	}
	if !row.ExpiresAt.Equal(expiresAt) {
		t.Errorf("successor expires_at = %v, want the lineage's %v", row.ExpiresAt, expiresAt)
	}

	time.Sleep(time.Until(expiresAt) + 50*time.Millisecond)
	if _, err := sharedDB.RotateRefreshToken(ctx, "l2", "l3"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("rotate past lineage expiry err = %v, want ErrNotFound", err)
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
	row, err := sharedDB.RotateRefreshToken(ctx, "h1", "h2")
	if err != nil {
		t.Fatalf("first rotate: %v", err)
	}
	if !row.ClaimAdmin || len(row.ClaimGroups) != 1 || row.IDToken != "idtok" {
		t.Fatalf("snapshot not carried forward: %+v", row)
	}

	// Replaying the now-rotated h1 is reuse → ErrRefreshReuse + lineage revoked.
	if _, err := sharedDB.RotateRefreshToken(ctx, "h1", "h3"); !errors.Is(err, store.ErrRefreshReuse) {
		t.Fatalf("reuse rotate err = %v, want ErrRefreshReuse", err)
	}
	// The successor h2 must now be revoked too (whole lineage killed).
	if _, err := sharedDB.RotateRefreshToken(ctx, "h2", "h4"); !errors.Is(err, store.ErrRefreshReuse) {
		t.Fatalf("h2 after lineage revoke err = %v, want ErrRefreshReuse", err)
	}
}

func TestRefreshToken_RotateUnknownAndExpired(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	u := newUser(t, ctx, "exp@x.test")

	if _, err := sharedDB.RotateRefreshToken(ctx, "nope", "x"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown rotate err = %v, want ErrNotFound", err)
	}

	if _, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "old", AuthMethod: "local",
		ExpiresAt: time.Now().Add(-time.Minute), // already expired
	}); err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	if _, err := sharedDB.RotateRefreshToken(ctx, "old", "new"); !errors.Is(err, store.ErrNotFound) {
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

	for _, hash := range []string{"live", "rev"} {
		if _, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
			UserID: u.ID, TokenHash: hash, AuthMethod: "local", ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatalf("create %s: %v", hash, err)
		}
	}
	for _, hash := range []string{"exp", "exp-rev"} {
		if _, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
			UserID: u.ID, TokenHash: hash, AuthMethod: "local", ExpiresAt: time.Now().Add(-time.Hour),
		}); err != nil {
			t.Fatalf("create %s: %v", hash, err)
		}
	}
	for _, hash := range []string{"rev", "exp-rev"} {
		if _, err := sharedDB.RevokeRefreshToken(ctx, hash); err != nil {
			t.Fatalf("revoke %s: %v", hash, err)
		}
	}

	n, err := sharedDB.DeleteExpiredRefreshTokens(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredRefreshTokens: %v", err)
	}
	if n != 2 {
		t.Errorf("deleted %d, want 2 (the expired rows, revoked or not)", n)
	}
	var left []string
	rows, err := sharedDB.Pool.Query(ctx, `SELECT token_hash FROM refresh_tokens ORDER BY token_hash`)
	if err != nil {
		t.Fatalf("query remaining: %v", err)
	}
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			t.Fatalf("scan: %v", err)
		}
		left = append(left, h)
	}
	if rows.Err() != nil || len(left) != 2 || left[0] != "live" || left[1] != "rev" {
		t.Errorf("remaining = %v (err %v), want [live rev]", left, rows.Err())
	}
}

func TestRefreshToken_ReuseDetectedAfterSweep(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	u := newUser(t, ctx, "sweep-reuse@x.test")

	if _, err := sharedDB.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "r1", AuthMethod: "local", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	if _, err := sharedDB.RotateRefreshToken(ctx, "r1", "r2"); err != nil {
		t.Fatalf("RotateRefreshToken: %v", err)
	}
	if _, err := sharedDB.DeleteExpiredRefreshTokens(ctx); err != nil {
		t.Fatalf("DeleteExpiredRefreshTokens: %v", err)
	}

	if _, err := sharedDB.RotateRefreshToken(ctx, "r1", "r3"); !errors.Is(err, store.ErrRefreshReuse) {
		t.Fatalf("replayed rotated token err = %v, want ErrRefreshReuse", err)
	}
	if _, err := sharedDB.RotateRefreshToken(ctx, "r2", "r4"); !errors.Is(err, store.ErrRefreshReuse) {
		t.Fatalf("successor after reuse err = %v, want ErrRefreshReuse (lineage revoked)", err)
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
		CodeHash: "expired", AccessToken: "a", RefreshToken: "r", ExpiresIn: 900, ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("create expired: %v", err)
	}
	n, err := sharedDB.DeleteExpiredHandoffCodes(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredHandoffCodes: %v", err)
	}
	if n != 1 {
		t.Errorf("deleted %d, want 1 (expired)", n)
	}
	if _, _, _, err := sharedDB.ConsumeHandoffCode(ctx, "live"); err != nil {
		t.Errorf("live code should survive the sweep: %v", err)
	}
}

func TestHandoffCode_ConsumeDropsTokens(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	if err := sharedDB.CreateHandoffCode(ctx, store.CreateHandoffCodeParams{
		CodeHash: "c", AccessToken: "acc", RefreshToken: "ref", ExpiresIn: 900, ExpiresAt: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatalf("CreateHandoffCode: %v", err)
	}
	if _, _, _, err := sharedDB.ConsumeHandoffCode(ctx, "c"); err != nil {
		t.Fatalf("ConsumeHandoffCode: %v", err)
	}
	var n int
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT count(*) FROM auth_handoff_codes WHERE access_token <> '' OR refresh_token <> ''`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("%d handoff rows still hold tokens after consume, want 0", n)
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

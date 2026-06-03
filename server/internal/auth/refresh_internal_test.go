package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/store"
)

// fakeRefreshStore is an in-memory RefreshStore for unit-testing RefreshManager
// without a database.
type fakeRefreshStore struct {
	created   []store.CreateRefreshTokenParams
	rotateOld string
	rotateNew string
	revoked   string
	rotateRow *store.RefreshToken
	rotateErr error
	revokeErr error
	createErr error
}

func (f *fakeRefreshStore) CreateRefreshToken(_ context.Context, p store.CreateRefreshTokenParams) (*store.RefreshToken, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = append(f.created, p)
	return &store.RefreshToken{ID: "rt", UserID: p.UserID}, nil
}

func (f *fakeRefreshStore) RotateRefreshToken(_ context.Context, oldHash, newHash string, _ time.Time) (*store.RefreshToken, error) {
	f.rotateOld, f.rotateNew = oldHash, newHash
	if f.rotateErr != nil {
		return nil, f.rotateErr
	}
	return f.rotateRow, nil
}

func (f *fakeRefreshStore) RevokeRefreshToken(_ context.Context, tokenHash string) (*store.RefreshToken, error) {
	f.revoked = tokenHash
	if f.revokeErr != nil {
		return nil, f.revokeErr
	}
	return &store.RefreshToken{ID: "rt"}, nil
}

func TestRefreshManager_Issue(t *testing.T) {
	fs := &fakeRefreshStore{}
	m := NewRefreshManager(fs, time.Hour)

	raw, err := m.Issue(context.Background(), RefreshIssueParams{
		UserID: "u1", AuthMethod: "oidc", ClaimGroups: []string{"g1"}, ClaimAdmin: true, IDToken: "idt",
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if raw == "" {
		t.Fatal("Issue should return a non-empty raw token")
	}
	if len(fs.created) != 1 {
		t.Fatalf("expected one CreateRefreshToken call, got %d", len(fs.created))
	}
	p := fs.created[0]
	// Only the hash is persisted — never the raw token.
	if p.TokenHash != hashToken(raw) {
		t.Errorf("stored hash %q does not match hashToken(raw)", p.TokenHash)
	}
	if p.TokenHash == raw {
		t.Error("the raw token must not be stored")
	}
	if p.UserID != "u1" || p.AuthMethod != "oidc" || !p.ClaimAdmin || p.IDToken != "idt" {
		t.Errorf("params not carried through: %+v", p)
	}
}

func TestRefreshManager_IssueDefaultsTTL(t *testing.T) {
	// A non-positive TTL falls back to the 30-day default (no panic, future expiry).
	fs := &fakeRefreshStore{}
	m := NewRefreshManager(fs, 0)
	if _, err := m.Issue(context.Background(), RefreshIssueParams{UserID: "u1", AuthMethod: "local"}); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if !fs.created[0].ExpiresAt.After(time.Now()) {
		t.Fatal("expiry should be in the future with the default TTL")
	}
}

func TestRefreshManager_Rotate(t *testing.T) {
	fs := &fakeRefreshStore{rotateRow: &store.RefreshToken{ID: "new", UserID: "u1"}}
	m := NewRefreshManager(fs, time.Hour)

	newRaw, row, err := m.Rotate(context.Background(), "old-raw")
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if newRaw == "" || newRaw == "old-raw" {
		t.Fatalf("Rotate should return a fresh raw token, got %q", newRaw)
	}
	if row == nil || row.ID != "new" {
		t.Fatalf("Rotate should return the successor row, got %+v", row)
	}
	if fs.rotateOld != hashToken("old-raw") || fs.rotateNew != hashToken(newRaw) {
		t.Errorf("Rotate must pass hashes, not raw tokens: old=%q new=%q", fs.rotateOld, fs.rotateNew)
	}
}

func TestRefreshManager_RotatePropagatesReuse(t *testing.T) {
	fs := &fakeRefreshStore{rotateErr: store.ErrRefreshReuse}
	m := NewRefreshManager(fs, time.Hour)
	if _, _, err := m.Rotate(context.Background(), "raw"); !errors.Is(err, store.ErrRefreshReuse) {
		t.Fatalf("Rotate err = %v, want ErrRefreshReuse", err)
	}
}

func TestRefreshManager_Revoke(t *testing.T) {
	fs := &fakeRefreshStore{}
	m := NewRefreshManager(fs, time.Hour)
	if _, err := m.Revoke(context.Background(), "raw"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if fs.revoked != hashToken("raw") {
		t.Errorf("Revoke must pass the hash, got %q", fs.revoked)
	}
}

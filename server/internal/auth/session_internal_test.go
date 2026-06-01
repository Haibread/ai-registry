package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/store"
)

// fakeSessionStore is an in-memory SessionStore keyed by token hash.
type fakeSessionStore struct {
	byHash map[string]*store.Session
}

func newFakeSessionStore() *fakeSessionStore {
	return &fakeSessionStore{byHash: map[string]*store.Session{}}
}

func (f *fakeSessionStore) CreateSession(_ context.Context, p store.CreateSessionParams) (*store.Session, error) {
	if _, ok := f.byHash[p.TokenHash]; ok {
		return nil, store.ErrConflict
	}
	s := &store.Session{
		ID:          "sess-" + p.TokenHash[:8],
		UserID:      p.UserID,
		AuthMethod:  p.AuthMethod,
		ClaimGroups: p.ClaimGroups,
		ClaimAdmin:  p.ClaimAdmin,
		CreatedAt:   time.Now(),
		ExpiresAt:   p.ExpiresAt,
	}
	f.byHash[p.TokenHash] = s
	return s, nil
}

func (f *fakeSessionStore) ActiveSessionByTokenHash(_ context.Context, h string) (*store.Session, error) {
	s, ok := f.byHash[h]
	if !ok || s.RevokedAt != nil || s.ExpiresAt.Before(time.Now()) {
		return nil, store.ErrNotFound
	}
	return s, nil
}

func (f *fakeSessionStore) RevokeSession(_ context.Context, h string) error {
	s, ok := f.byHash[h]
	if !ok || s.RevokedAt != nil {
		return store.ErrNotFound
	}
	now := time.Now()
	s.RevokedAt = &now
	return nil
}

func testManager() (*SessionManager, *fakeSessionStore) {
	fs := newFakeSessionStore()
	return NewSessionManager(fs, SessionConfig{
		CookieName: "sess",
		TTL:        time.Hour,
		Secure:     true,
		SameSite:   http.SameSiteLaxMode,
	}), fs
}

func TestSessionManager_CookieAccessors(t *testing.T) {
	sm, _ := testManager()
	if sm.CookieName() != "sess" {
		t.Fatalf("CookieName = %q, want sess", sm.CookieName())
	}
	if !sm.CookieSecure() {
		t.Fatal("CookieSecure should be true")
	}
	c := sm.Cookie("tok-123")
	if c.Name != "sess" || c.Value != "tok-123" || !c.HttpOnly || !c.Secure || c.MaxAge <= 0 {
		t.Fatalf("unexpected session cookie: %+v", c)
	}
	cleared := sm.ClearCookie()
	if cleared.Name != "sess" || cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Fatalf("ClearCookie should expire the cookie: %+v", cleared)
	}

	withCookie := httptest.NewRequest(http.MethodGet, "/", nil)
	withCookie.AddCookie(&http.Cookie{Name: "sess", Value: "abc"})
	if got := sm.TokenFromRequest(withCookie); got != "abc" {
		t.Fatalf("TokenFromRequest = %q, want abc", got)
	}
	if got := sm.TokenFromRequest(httptest.NewRequest(http.MethodGet, "/", nil)); got != "" {
		t.Fatalf("TokenFromRequest with no cookie = %q, want empty", got)
	}
}

func TestHashToken_DeterministicAndDistinct(t *testing.T) {
	a, b := hashToken("abc"), hashToken("abc")
	if a != b {
		t.Fatal("hashToken must be deterministic")
	}
	if a == hashToken("abd") {
		t.Fatal("different tokens must hash differently")
	}
	if len(a) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(a))
	}
}

func TestNewToken_UniqueAndNonEmpty(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		tok, err := newToken()
		if err != nil {
			t.Fatalf("newToken: %v", err)
		}
		if tok == "" {
			t.Fatal("token must not be empty")
		}
		if seen[tok] {
			t.Fatal("token collision")
		}
		seen[tok] = true
	}
}

func TestSessionManager_IssueResolveRevoke(t *testing.T) {
	m, fs := testManager()
	ctx := context.Background()

	token, err := m.Issue(ctx, IssueParams{
		UserID:      "u1",
		AuthMethod:  "oidc",
		ClaimGroups: []string{"team-a"},
		ClaimAdmin:  true,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// The raw token must NOT be a key in the store — only its hash is.
	if _, ok := fs.byHash[token]; ok {
		t.Fatal("raw token must not be stored; only its hash")
	}
	if _, ok := fs.byHash[hashToken(token)]; !ok {
		t.Fatal("token hash should be stored")
	}

	got, err := m.Resolve(ctx, token)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.UserID != "u1" || !got.ClaimAdmin || len(got.ClaimGroups) != 1 {
		t.Fatalf("resolved session mismatch: %+v", got)
	}

	if _, err := m.Resolve(ctx, "not-a-real-token"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown token, got %v", err)
	}

	if err := m.Revoke(ctx, token); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := m.Resolve(ctx, token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after revoke, got %v", err)
	}
}

func TestSessionManager_CookieAttributes(t *testing.T) {
	m, _ := testManager()
	c := m.Cookie("tok")
	if c.Name != "sess" || c.Value != "tok" {
		t.Fatalf("unexpected cookie name/value: %+v", c)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("cookie security attributes wrong: %+v", c)
	}
	if c.MaxAge <= 0 {
		t.Fatalf("expected positive MaxAge, got %d", c.MaxAge)
	}

	cleared := m.ClearCookie()
	if cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Fatalf("ClearCookie should expire the cookie: %+v", cleared)
	}
}

func TestSessionManager_TokenFromRequest(t *testing.T) {
	m, _ := testManager()
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	if got := m.TokenFromRequest(r); got != "" {
		t.Fatalf("expected empty token with no cookie, got %q", got)
	}
	r.AddCookie(&http.Cookie{Name: "sess", Value: "tok"})
	if got := m.TokenFromRequest(r); got != "tok" {
		t.Fatalf("expected tok, got %q", got)
	}
}

func TestParseSameSite(t *testing.T) {
	cases := map[string]http.SameSite{
		"lax":     http.SameSiteLaxMode,
		"strict":  http.SameSiteStrictMode,
		"none":    http.SameSiteNoneMode,
		"":        http.SameSiteLaxMode,
		"garbage": http.SameSiteLaxMode,
	}
	for in, want := range cases {
		if got := ParseSameSite(in); got != want {
			t.Fatalf("ParseSameSite(%q) = %v, want %v", in, got, want)
		}
	}
}

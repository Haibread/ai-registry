package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

// fakeLoginStore is an in-memory localLoginStore for unit-testing the login
// handler without a database.
type fakeLoginStore struct {
	creds      map[string]*store.Credentials // keyed by normalised email
	touchCalls int
}

func (f *fakeLoginStore) CredentialsByEmail(_ context.Context, email string) (*store.Credentials, error) {
	if c, ok := f.creds[email]; ok {
		return c, nil
	}
	return nil, store.ErrNotFound
}

func (f *fakeLoginStore) TouchLastSeen(_ context.Context, _ string) error {
	f.touchCalls++
	return nil
}

// fakeSessionStore is an in-memory auth.SessionStore for the login tests.
type fakeSessionStore struct {
	byHash map[string]*store.Session
}

func (f *fakeSessionStore) CreateSession(_ context.Context, p store.CreateSessionParams) (*store.Session, error) {
	s := &store.Session{ID: "s1", UserID: p.UserID, AuthMethod: p.AuthMethod, IDToken: p.IDToken, ExpiresAt: p.ExpiresAt}
	f.byHash[p.TokenHash] = s
	return s, nil
}

func (f *fakeSessionStore) ActiveSessionByTokenHash(_ context.Context, h string) (*store.Session, error) {
	if s, ok := f.byHash[h]; ok {
		return s, nil
	}
	return nil, store.ErrNotFound
}

func (f *fakeSessionStore) RevokeSession(_ context.Context, h string) error {
	if _, ok := f.byHash[h]; ok {
		delete(f.byHash, h)
		return nil
	}
	return store.ErrNotFound
}

func testSessions() (*auth.SessionManager, *fakeSessionStore) {
	fs := &fakeSessionStore{byHash: map[string]*store.Session{}}
	return auth.NewSessionManager(fs, auth.SessionConfig{CookieName: "sess", TTL: time.Hour}), fs
}

func newAuthHandlers(t *testing.T, st *fakeLoginStore, localEnabled bool) (*handlers.AuthHandlers, *fakeSessionStore) {
	t.Helper()
	sm, fs := testSessions()
	return handlers.NewAuthHandlers(sm, st, localEnabled), fs
}

func postLogin(h *handlers.AuthHandlers, email, password string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	return rec
}

func sessionCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func storeWithUser(t *testing.T, email, password string, disabled bool) *fakeLoginStore {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return &fakeLoginStore{creds: map[string]*store.Credentials{
		email: {UserID: "u1", PasswordHash: hash, Disabled: disabled},
	}}
}

func TestLogin_Success(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "hunter2hunter2", false)
	h, fs := newAuthHandlers(t, st, true)

	rec := postLogin(h, "Dev@Example.com", "hunter2hunter2") // mixed-case email
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	c := sessionCookie(rec, "sess")
	if c == nil || c.Value == "" {
		t.Fatal("expected a session cookie to be set")
	}
	if !c.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
	if len(fs.byHash) != 1 {
		t.Errorf("expected one session row, got %d", len(fs.byHash))
	}
	if st.touchCalls != 1 {
		t.Errorf("TouchLastSeen calls = %d, want 1", st.touchCalls)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "correct-password", false)
	h, _ := newAuthHandlers(t, st, true)
	if rec := postLogin(h, "dev@example.com", "wrong-password"); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	h, _ := newAuthHandlers(t, &fakeLoginStore{creds: map[string]*store.Credentials{}}, true)
	if rec := postLogin(h, "ghost@example.com", "whatever"); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_Disabled(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "pw-pw-pw-pw", true)
	h, _ := newAuthHandlers(t, st, true)
	if rec := postLogin(h, "dev@example.com", "pw-pw-pw-pw"); rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for disabled account", rec.Code)
	}
}

// A disabled account must be indistinguishable from a wrong password when the
// caller does NOT know the password — otherwise the distinct 403 is an
// account-enumeration oracle.
func TestLogin_Disabled_WrongPassword_Uniform(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "the-real-password", true)
	h, _ := newAuthHandlers(t, st, true)
	if rec := postLogin(h, "dev@example.com", "wrong-password"); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for disabled account with a wrong password", rec.Code)
	}
}

func TestLogin_NoLocalPassword(t *testing.T) {
	// OIDC-only / invited user: row exists but has no password hash.
	st := &fakeLoginStore{creds: map[string]*store.Credentials{
		"oidc@example.com": {UserID: "u2", PasswordHash: "", Disabled: false},
	}}
	h, _ := newAuthHandlers(t, st, true)
	if rec := postLogin(h, "oidc@example.com", "anything"); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (no local password)", rec.Code)
	}
}

func TestLogin_LocalDisabled404(t *testing.T) {
	h, _ := newAuthHandlers(t, &fakeLoginStore{creds: map[string]*store.Credentials{}}, false)
	if rec := postLogin(h, "dev@example.com", "pw"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when local login is disabled", rec.Code)
	}
}

func TestLogin_Lockout(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "the-real-password", false)
	h, _ := newAuthHandlers(t, st, true)

	for i := 0; i < 5; i++ {
		if rec := postLogin(h, "dev@example.com", "bad"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i, rec.Code)
		}
	}
	rec := postLogin(h, "dev@example.com", "the-real-password")
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 after lockout", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("429 response should carry a Retry-After header")
	}
}

// fakeOIDCStore satisfies the OIDC callback store; the Logout handler never
// touches it, so every method is a stub.
type fakeOIDCStore struct{}

func (fakeOIDCStore) GetUserByID(context.Context, string) (*store.User, error) {
	return nil, store.ErrNotFound
}
func (fakeOIDCStore) GetUserBySubject(context.Context, string) (*store.User, error) {
	return nil, store.ErrNotFound
}
func (fakeOIDCStore) GetUserByEmail(context.Context, string) (*store.User, error) {
	return nil, store.ErrNotFound
}
func (fakeOIDCStore) CreateUser(context.Context, store.CreateUserParams) (*store.User, error) {
	return nil, store.ErrNotFound
}
func (fakeOIDCStore) BindSubject(context.Context, string, string) error { return nil }
func (fakeOIDCStore) TouchLastSeen(context.Context, string) error       { return nil }

// newLogoutBroker stands up a minimal IdP (discovery only) advertising an
// end_session_endpoint, and returns a broker pointed at it.
func newLogoutBroker(t *testing.T) (*auth.OIDCBroker, string) {
	t.Helper()
	mux := http.NewServeMux()
	var issuer string
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 issuer,
			"authorization_endpoint": issuer + "/auth",
			"token_endpoint":         issuer + "/token",
			"end_session_endpoint":   issuer + "/logout",
		})
	})
	srv := httptest.NewServer(mux)
	issuer = srv.URL
	t.Cleanup(srv.Close)
	b, err := auth.NewOIDCBroker(context.Background(), auth.OIDCConfig{
		Issuer: srv.URL, ClientID: "cid", ClientSecret: "secret",
	})
	if err != nil {
		t.Fatalf("NewOIDCBroker: %v", err)
	}
	return b, srv.URL
}

func TestOIDCLogout_OIDCSession_RedirectsThroughIdP(t *testing.T) {
	broker, idpURL := newLogoutBroker(t)
	sm, fs := testSessions()
	h := handlers.NewOIDCAuthHandlers(broker, sm, fakeOIDCStore{}, false, "/", "http://app.example/")

	token, err := sm.Issue(context.Background(), auth.IssueParams{
		UserID: "u1", AuthMethod: "oidc", IDToken: "idtok-123",
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "sess", Value: token})
	rec := httptest.NewRecorder()
	h.Logout(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, idpURL+"/logout") {
		t.Fatalf("Location = %q, want IdP end-session prefix", loc)
	}
	if !strings.Contains(loc, "id_token_hint=idtok-123") {
		t.Errorf("Location missing id_token_hint: %q", loc)
	}
	if !strings.Contains(loc, "post_logout_redirect_uri=") || !strings.Contains(loc, "client_id=cid") {
		t.Errorf("Location missing post_logout_redirect_uri/client_id: %q", loc)
	}
	if len(fs.byHash) != 0 {
		t.Errorf("session should be revoked, %d remain", len(fs.byHash))
	}
	if c := sessionCookie(rec, "sess"); c == nil || c.MaxAge >= 0 {
		t.Errorf("cookie should be cleared, got %+v", c)
	}
}

func TestOIDCLogout_LocalSession_NoIdPBounce(t *testing.T) {
	broker, _ := newLogoutBroker(t)
	sm, _ := testSessions()
	h := handlers.NewOIDCAuthHandlers(broker, sm, fakeOIDCStore{}, false, "/", "http://app.example/")

	token, err := sm.Issue(context.Background(), auth.IssueParams{UserID: "u1", AuthMethod: "local"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "sess", Value: token})
	rec := httptest.NewRecorder()
	h.Logout(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "http://app.example/" {
		t.Fatalf("local logout Location = %q, want the SPA origin (no IdP bounce)", loc)
	}
}

func TestOIDCLogin_RedirectsToIdPWithTxnCookie(t *testing.T) {
	broker, idpURL := newLogoutBroker(t)
	sm, _ := testSessions()
	h := handlers.NewOIDCAuthHandlers(broker, sm, fakeOIDCStore{}, false, "/", "http://app.example/")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/login", nil)
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, idpURL+"/auth") {
		t.Fatalf("Location = %q, want the IdP authorize endpoint", loc)
	}
	for _, want := range []string{"response_type=code", "code_challenge=", "code_challenge_method=S256", "state=", "nonce="} {
		if !strings.Contains(loc, want) {
			t.Errorf("authorize URL missing %q: %s", want, loc)
		}
	}
	// The short-lived login-transaction cookie must be set so the callback can
	// validate state/nonce and complete PKCE.
	if c := sessionCookie(rec, "ai_registry_oidc_txn"); c == nil || c.Value == "" {
		t.Fatalf("expected the OIDC txn cookie to be set, got %+v", c)
	}
}

func TestOIDCLogin_NoBrokerReturns404(t *testing.T) {
	sm, _ := testSessions()
	h := handlers.NewOIDCAuthHandlers(nil, sm, fakeOIDCStore{}, false, "/", "http://app.example/")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/login", nil)
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when OIDC is not configured", rec.Code)
	}
}

func TestOIDCCallback_NoBrokerReturns404(t *testing.T) {
	sm, _ := testSessions()
	h := handlers.NewOIDCAuthHandlers(nil, sm, fakeOIDCStore{}, false, "/", "http://app.example/")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?code=x&state=y", nil)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when OIDC is not configured", rec.Code)
	}
}

func TestLogout_RevokesAndClears(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "hunter2hunter2", false)
	h, fs := newAuthHandlers(t, st, true)

	loginRec := postLogin(h, "dev@example.com", "hunter2hunter2")
	c := sessionCookie(loginRec, "sess")
	if c == nil {
		t.Fatal("login should set a session cookie")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.Logout(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", rec.Code)
	}
	if len(fs.byHash) != 0 {
		t.Errorf("logout should revoke (delete) the session, %d remain", len(fs.byHash))
	}
	cleared := sessionCookie(rec, "sess")
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("logout should clear the cookie, got %+v", cleared)
	}
}

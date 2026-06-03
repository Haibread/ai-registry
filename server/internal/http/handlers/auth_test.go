package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

// fakeAuthStore is an in-memory implementation of every store slice the auth
// handlers and the refresh manager need: credentials, users, refresh tokens,
// OIDC login transactions, and handoff codes.
type fakeAuthStore struct {
	creds      map[string]*store.Credentials  // keyed by normalised email
	users      map[string]*store.User         // keyed by id
	refresh    map[string]*store.RefreshToken // keyed by token hash
	requests   map[string]store.CreateOIDCAuthRequestParams
	handoffs   map[string]store.CreateHandoffCodeParams
	touchCalls int
	revokedAll []string
	seq        int
}

func newFakeAuthStore() *fakeAuthStore {
	return &fakeAuthStore{
		creds:    map[string]*store.Credentials{},
		users:    map[string]*store.User{},
		refresh:  map[string]*store.RefreshToken{},
		requests: map[string]store.CreateOIDCAuthRequestParams{},
		handoffs: map[string]store.CreateHandoffCodeParams{},
	}
}

// ── credentials / users ──────────────────────────────────────────────────────

func (f *fakeAuthStore) CredentialsByEmail(_ context.Context, email string) (*store.Credentials, error) {
	if c, ok := f.creds[email]; ok {
		return c, nil
	}
	return nil, store.ErrNotFound
}

func (f *fakeAuthStore) GetUserByID(_ context.Context, id string) (*store.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, store.ErrNotFound
}

func (f *fakeAuthStore) TouchLastSeen(_ context.Context, _ string) error {
	f.touchCalls++
	return nil
}

func (f *fakeAuthStore) RevokeAllRefreshTokensForUser(_ context.Context, userID string) (int64, error) {
	f.revokedAll = append(f.revokedAll, userID)
	var n int64
	for _, t := range f.refresh {
		if t.UserID == userID && t.RevokedAt == nil {
			now := time.Now()
			t.RevokedAt = &now
			n++
		}
	}
	return n, nil
}

// ── refresh tokens ───────────────────────────────────────────────────────────

func (f *fakeAuthStore) CreateRefreshToken(_ context.Context, p store.CreateRefreshTokenParams) (*store.RefreshToken, error) {
	f.seq++
	t := &store.RefreshToken{
		ID: p.TokenHash[:8], UserID: p.UserID, AuthMethod: p.AuthMethod,
		ClaimGroups: p.ClaimGroups, ClaimAdmin: p.ClaimAdmin, IDToken: p.IDToken,
		ExpiresAt: p.ExpiresAt,
	}
	f.refresh[p.TokenHash] = t
	return t, nil
}

func (f *fakeAuthStore) RotateRefreshToken(_ context.Context, oldHash, newHash string, newExpiresAt time.Time) (*store.RefreshToken, error) {
	old, ok := f.refresh[oldHash]
	if !ok {
		return nil, store.ErrNotFound
	}
	if old.RevokedAt != nil {
		// Reuse: revoke the lineage.
		for _, t := range f.refresh {
			if t.UserID == old.UserID && t.RevokedAt == nil {
				now := time.Now()
				t.RevokedAt = &now
			}
		}
		return nil, store.ErrRefreshReuse
	}
	if !old.ExpiresAt.After(time.Now()) {
		return nil, store.ErrNotFound
	}
	now := time.Now()
	old.RevokedAt = &now
	t := &store.RefreshToken{
		ID: newHash[:8], UserID: old.UserID, AuthMethod: old.AuthMethod,
		ClaimGroups: old.ClaimGroups, ClaimAdmin: old.ClaimAdmin, IDToken: old.IDToken,
		ExpiresAt: newExpiresAt,
	}
	f.refresh[newHash] = t
	return t, nil
}

func (f *fakeAuthStore) RevokeRefreshToken(_ context.Context, tokenHash string) (*store.RefreshToken, error) {
	t, ok := f.refresh[tokenHash]
	if !ok || t.RevokedAt != nil {
		return nil, store.ErrNotFound
	}
	now := time.Now()
	t.RevokedAt = &now
	return t, nil
}

// ── OIDC login transactions + handoff codes ──────────────────────────────────

func (f *fakeAuthStore) CreateOIDCAuthRequest(_ context.Context, p store.CreateOIDCAuthRequestParams) error {
	f.requests[p.StateHash] = p
	return nil
}

func (f *fakeAuthStore) ConsumeOIDCAuthRequest(_ context.Context, stateHash string) (string, string, error) {
	p, ok := f.requests[stateHash]
	if !ok {
		return "", "", store.ErrNotFound
	}
	delete(f.requests, stateHash)
	return p.Nonce, p.CodeVerifier, nil
}

func (f *fakeAuthStore) CreateHandoffCode(_ context.Context, p store.CreateHandoffCodeParams) error {
	f.handoffs[p.CodeHash] = p
	return nil
}

func (f *fakeAuthStore) ConsumeHandoffCode(_ context.Context, codeHash string) (string, string, int, error) {
	p, ok := f.handoffs[codeHash]
	if !ok {
		return "", "", 0, store.ErrNotFound
	}
	delete(f.handoffs, codeHash)
	return p.AccessToken, p.RefreshToken, p.ExpiresIn, nil
}

// ── PrincipalStore (for the OIDC callback) ───────────────────────────────────

func (f *fakeAuthStore) GetUserBySubject(context.Context, string) (*store.User, error) {
	return nil, store.ErrNotFound
}
func (f *fakeAuthStore) GetUserByEmail(context.Context, string) (*store.User, error) {
	return nil, store.ErrNotFound
}
func (f *fakeAuthStore) CreateUser(_ context.Context, p store.CreateUserParams) (*store.User, error) {
	return &store.User{ID: "jit-" + p.Subject, Email: p.Email, Subject: p.Subject}, nil
}
func (f *fakeAuthStore) BindSubject(context.Context, string, string) error { return nil }

// ── helpers ──────────────────────────────────────────────────────────────────

func testAuthority(t *testing.T) *auth.TokenAuthority {
	t.Helper()
	ta, _, err := auth.NewTokenAuthority("", "", "test-issuer", time.Minute)
	if err != nil {
		t.Fatalf("NewTokenAuthority: %v", err)
	}
	return ta
}

func newAuthHandlers(t *testing.T, st *fakeAuthStore, localEnabled bool) (*handlers.AuthHandlers, *auth.TokenAuthority) {
	t.Helper()
	ta := testAuthority(t)
	rm := auth.NewRefreshManager(st, time.Hour)
	return handlers.NewAuthHandlers(ta, rm, st, localEnabled), ta
}

func storeWithUser(t *testing.T, email, password string, disabled bool) *fakeAuthStore {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	st := newFakeAuthStore()
	st.creds[email] = &store.Credentials{UserID: "u1", PasswordHash: hash, Disabled: disabled}
	st.users["u1"] = &store.User{ID: "u1", Email: email, Disabled: disabled}
	return st
}

func postLogin(h *handlers.AuthHandlers, email, password string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	return rec
}

type tokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int    `json:"expiresIn"`
}

func decodePair(t *testing.T, rec *httptest.ResponseRecorder) tokenPair {
	t.Helper()
	var p tokenPair
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decoding token pair: %v; body: %s", err, rec.Body.String())
	}
	return p
}

// ── local login ──────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "hunter2hunter2", false)
	st.users["u1"].IsServerAdmin = true
	h, ta := newAuthHandlers(t, st, true)

	rec := postLogin(h, "Dev@Example.com", "hunter2hunter2") // mixed-case email
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	p := decodePair(t, rec)
	if p.AccessToken == "" || p.RefreshToken == "" || p.TokenType != "Bearer" || p.ExpiresIn <= 0 {
		t.Fatalf("incomplete token pair: %+v", p)
	}
	claims, err := ta.Verify(p.AccessToken)
	if err != nil {
		t.Fatalf("minted access token does not verify: %v", err)
	}
	if claims.Subject != "u1" || !claims.SrvAdmin || claims.AuthMethod != "local" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if len(st.refresh) != 1 {
		t.Errorf("expected one refresh token row, got %d", len(st.refresh))
	}
	if st.touchCalls != 1 {
		t.Errorf("TouchLastSeen calls = %d, want 1", st.touchCalls)
	}
	if c := rec.Result().Cookies(); len(c) != 0 {
		t.Errorf("bearer login must not set any cookie, got %d", len(c))
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
	h, _ := newAuthHandlers(t, newFakeAuthStore(), true)
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

func TestLogin_Disabled_WrongPassword_Uniform(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "the-real-password", true)
	h, _ := newAuthHandlers(t, st, true)
	if rec := postLogin(h, "dev@example.com", "wrong-password"); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for disabled account with a wrong password", rec.Code)
	}
}

func TestLogin_NoLocalPassword(t *testing.T) {
	st := newFakeAuthStore()
	st.creds["oidc@example.com"] = &store.Credentials{UserID: "u2", PasswordHash: "", Disabled: false}
	h, _ := newAuthHandlers(t, st, true)
	if rec := postLogin(h, "oidc@example.com", "anything"); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (no local password)", rec.Code)
	}
}

func TestLogin_LocalDisabled404(t *testing.T) {
	h, _ := newAuthHandlers(t, newFakeAuthStore(), false)
	if rec := postLogin(h, "dev@example.com", "pw"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when local login is disabled", rec.Code)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	h, _ := newAuthHandlers(t, newFakeAuthStore(), true)
	if rec := postLogin(h, "", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when email/password are missing", rec.Code)
	}
}

func TestRefresh_MissingToken(t *testing.T) {
	h, _ := newAuthHandlers(t, newFakeAuthStore(), true)
	if rec := postRefresh(h, ""); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when refreshToken is missing", rec.Code)
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

// ── refresh ──────────────────────────────────────────────────────────────────

func postRefresh(h *handlers.AuthHandlers, refreshToken string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"refreshToken": refreshToken})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Refresh(rec, req)
	return rec
}

func TestRefresh_RotatesAndIssuesNewPair(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "hunter2hunter2", false)
	h, ta := newAuthHandlers(t, st, true)

	login := decodePair(t, postLogin(h, "dev@example.com", "hunter2hunter2"))

	rec := postRefresh(h, login.RefreshToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	next := decodePair(t, rec)
	if next.RefreshToken == login.RefreshToken {
		t.Fatal("refresh must rotate the token")
	}
	if _, err := ta.Verify(next.AccessToken); err != nil {
		t.Fatalf("refreshed access token does not verify: %v", err)
	}
}

func TestRefresh_ReuseRevokesLineage(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "hunter2hunter2", false)
	h, _ := newAuthHandlers(t, st, true)

	login := decodePair(t, postLogin(h, "dev@example.com", "hunter2hunter2"))
	if rec := postRefresh(h, login.RefreshToken); rec.Code != http.StatusOK {
		t.Fatalf("first refresh status = %d, want 200", rec.Code)
	}
	// Replaying the now-rotated token is theft → 401.
	if rec := postRefresh(h, login.RefreshToken); rec.Code != http.StatusUnauthorized {
		t.Fatalf("reused refresh status = %d, want 401", rec.Code)
	}
}

func TestRefresh_DisabledUserRejected(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "hunter2hunter2", false)
	h, _ := newAuthHandlers(t, st, true)

	login := decodePair(t, postLogin(h, "dev@example.com", "hunter2hunter2"))
	st.users["u1"].Disabled = true // disabled after login

	rec := postRefresh(h, login.RefreshToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("refresh of disabled account status = %d, want 403", rec.Code)
	}
	if len(st.revokedAll) == 0 {
		t.Error("disabling at refresh time should revoke all refresh tokens")
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "hunter2hunter2", false)
	h, _ := newAuthHandlers(t, st, true)
	if rec := postRefresh(h, "no-such-refresh-token"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for an unknown refresh token", rec.Code)
	}
}

// ── OIDC handlers ────────────────────────────────────────────────────────────

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

func newOIDCHandlers(t *testing.T, broker *auth.OIDCBroker, st *fakeAuthStore) *handlers.OIDCAuthHandlers {
	t.Helper()
	ta := testAuthority(t)
	rm := auth.NewRefreshManager(st, time.Hour)
	return handlers.NewOIDCAuthHandlers(broker, ta, rm, st, "/", "http://app.example/")
}

func TestOIDCLogin_RedirectsToIdP_NoCookie(t *testing.T) {
	broker, idpURL := newLogoutBroker(t)
	st := newFakeAuthStore()
	h := newOIDCHandlers(t, broker, st)

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
	if c := rec.Result().Cookies(); len(c) != 0 {
		t.Errorf("login must not set a cookie anymore, got %d", len(c))
	}
	if len(st.requests) != 1 {
		t.Errorf("login should persist exactly one auth request, got %d", len(st.requests))
	}
}

func TestOIDCLogin_NoBrokerReturns404(t *testing.T) {
	h := newOIDCHandlers(t, nil, newFakeAuthStore())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/login", nil)
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when OIDC is not configured", rec.Code)
	}
}

func TestOIDCCallback_NoBrokerReturns404(t *testing.T) {
	h := newOIDCHandlers(t, nil, newFakeAuthStore())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?code=x&state=y", nil)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when OIDC is not configured", rec.Code)
	}
}

// newCallbackIdP stands up a minimal RS256 IdP and returns a broker pointed at
// it. The token endpoint always issues an id_token echoing the given nonce (so a
// callback whose stored auth-request carries the same nonce completes the flow)
// and the given realm roles under realm_access.roles.
func newCallbackIdP(t *testing.T, nonce string, realmRoles []string) *auth.OIDCBroker {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	var issuer string
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 issuer,
			"authorization_endpoint": issuer + "/auth",
			"token_endpoint":         issuer + "/token",
			"jwks_uri":               issuer + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		pub := key.Public().(*rsa.PublicKey)
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"iss": issuer, "aud": "cid", "exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Unix(), "sub": "sub-1", "email": "alice@example.com",
			"email_verified": true, "nonce": nonce, "groups": []string{"team-a"},
			"realm_access": map[string]any{"roles": realmRoles},
		})
		tok.Header["kid"] = "k1"
		signed, _ := tok.SignedString(key)
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": signed, "token_type": "Bearer"})
	})
	srv := httptest.NewServer(mux)
	issuer = srv.URL
	t.Cleanup(srv.Close)
	b, err := auth.NewOIDCBroker(context.Background(), auth.OIDCConfig{
		Issuer: srv.URL, ClientID: "cid", ClientSecret: "secret", RedirectURL: "https://app/cb",
	})
	if err != nil {
		t.Fatalf("NewOIDCBroker: %v", err)
	}
	return b
}

func TestOIDCCallback_HappyPath_MintsHandoffCode(t *testing.T) {
	const nonce = "n-cb"
	st := newFakeAuthStore()
	h := newOIDCHandlers(t, newCallbackIdP(t, nonce, nil), st)

	state := "state-xyz"
	st.requests[auth.HashToken(state)] = store.CreateOIDCAuthRequestParams{
		StateHash: auth.HashToken(state), Nonce: nonce, CodeVerifier: "verifier",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?code=abc&state="+state, nil)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "#code=") {
		t.Fatalf("Location should carry a one-time handoff code in the fragment: %q", loc)
	}
	if len(st.handoffs) != 1 {
		t.Fatalf("expected one handoff code stored, got %d", len(st.handoffs))
	}
	if len(st.requests) != 0 {
		t.Errorf("the login transaction should have been consumed, %d remain", len(st.requests))
	}
}

// TestOIDCCallback_AdminClaim_MintsServerAdminToken ties the OIDC role mapping to
// real authorization: an id_token carrying the admin realm role must produce a
// registry access token whose srvAdmin claim is set.
func TestOIDCCallback_AdminClaim_MintsServerAdminToken(t *testing.T) {
	const nonce = "n-admin"
	st := newFakeAuthStore()
	ta := testAuthority(t)
	broker := newCallbackIdP(t, nonce, []string{"admin"})
	h := handlers.NewOIDCAuthHandlers(broker, ta, auth.NewRefreshManager(st, time.Hour), st, "/", "http://app.example/")

	state := "state-admin"
	st.requests[auth.HashToken(state)] = store.CreateOIDCAuthRequestParams{
		StateHash: auth.HashToken(state), Nonce: nonce, CodeVerifier: "verifier",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?code=abc&state="+state, nil)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body: %s", rec.Code, rec.Body.String())
	}

	// Pull the minted access token out of the handoff and inspect its claims.
	if len(st.handoffs) != 1 {
		t.Fatalf("expected one handoff code, got %d", len(st.handoffs))
	}
	var access string
	for _, hc := range st.handoffs {
		access = hc.AccessToken
	}
	claims, err := ta.Verify(access)
	if err != nil {
		t.Fatalf("minted access token does not verify: %v", err)
	}
	if !claims.SrvAdmin {
		t.Error("an OIDC admin-role login must mint a token with srvAdmin=true")
	}
	if claims.AuthMethod != "oidc" {
		t.Errorf("authMethod = %q, want oidc", claims.AuthMethod)
	}
}

func TestOIDCCallback_UnknownStateReturns400(t *testing.T) {
	broker, _ := newLogoutBroker(t)
	h := newOIDCHandlers(t, broker, newFakeAuthStore())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?code=x&state=unknown", nil)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unknown login transaction", rec.Code)
	}
}

func TestOIDCExchange_ConsumesHandoffCode(t *testing.T) {
	broker, _ := newLogoutBroker(t)
	st := newFakeAuthStore()
	h := newOIDCHandlers(t, broker, st)

	// Seed a handoff code as the callback would.
	code := "one-time-code"
	st.handoffs[auth.HashToken(code)] = store.CreateHandoffCodeParams{
		CodeHash: auth.HashToken(code), AccessToken: "acc", RefreshToken: "ref", ExpiresIn: 900,
	}

	body, _ := json.Marshal(map[string]string{"code": code})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oidc/exchange", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Exchange(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	p := decodePair(t, rec)
	if p.AccessToken != "acc" || p.RefreshToken != "ref" || p.ExpiresIn != 900 {
		t.Fatalf("unexpected handoff payload: %+v", p)
	}
	// Single use.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oidc/exchange", bytes.NewReader(mustJSON(map[string]string{"code": code})))
	h.Exchange(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("replayed code status = %d, want 401", rec2.Code)
	}
}

func TestOIDCLogout_OIDCSession_ReturnsEndSessionURL(t *testing.T) {
	broker, idpURL := newLogoutBroker(t)
	st := newFakeAuthStore()
	h := newOIDCHandlers(t, broker, st)
	rm := auth.NewRefreshManager(st, time.Hour)

	raw, err := rm.Issue(context.Background(), auth.RefreshIssueParams{
		UserID: "u1", AuthMethod: "oidc", IDToken: "idtok-123",
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"refreshToken": raw})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Logout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		OIDCLogoutURL string `json:"oidcLogoutUrl"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !strings.HasPrefix(resp.OIDCLogoutURL, idpURL+"/logout") {
		t.Fatalf("oidcLogoutUrl = %q, want IdP end-session prefix", resp.OIDCLogoutURL)
	}
	if !strings.Contains(resp.OIDCLogoutURL, "id_token_hint=idtok-123") {
		t.Errorf("end-session URL missing id_token_hint: %q", resp.OIDCLogoutURL)
	}
}

func TestOIDCLogout_LocalSession_NoEndSessionURL(t *testing.T) {
	broker, _ := newLogoutBroker(t)
	st := newFakeAuthStore()
	h := newOIDCHandlers(t, broker, st)
	rm := auth.NewRefreshManager(st, time.Hour)

	raw, err := rm.Issue(context.Background(), auth.RefreshIssueParams{UserID: "u1", AuthMethod: "local"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"refreshToken": raw})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Logout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		OIDCLogoutURL string `json:"oidcLogoutUrl"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.OIDCLogoutURL != "" {
		t.Fatalf("local logout should not return an end-session URL, got %q", resp.OIDCLogoutURL)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestJWKS(t *testing.T) {
	ta := testAuthority(t)
	rec := httptest.NewRecorder()
	handlers.JWKS(ta)(rec, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decoding JWKS: %v", err)
	}
	if len(doc.Keys) != 1 || doc.Keys[0].Kty != "OKP" || doc.Keys[0].Kid == "" {
		t.Fatalf("unexpected JWKS payload: %+v", doc.Keys)
	}
}

func TestJWKS_NoAuthority503(t *testing.T) {
	rec := httptest.NewRecorder()
	handlers.JWKS(nil)(rec, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when no token authority is configured", rec.Code)
	}
}

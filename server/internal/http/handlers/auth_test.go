package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
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

func testLocalIssuer(t *testing.T) *auth.LocalIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	li, err := auth.NewLocalIssuer(string(pemBytes), time.Hour)
	if err != nil {
		t.Fatalf("NewLocalIssuer: %v", err)
	}
	return li
}

func postLogin(h *handlers.AuthHandlers, email, password string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	return rec
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
	h := handlers.NewAuthHandlers(testLocalIssuer(t), st)

	rec := postLogin(h, "Dev@Example.com", "hunter2hunter2") // mixed-case email
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.AccessToken == "" || resp.TokenType != "Bearer" || resp.ExpiresIn <= 0 {
		t.Errorf("unexpected response: %+v", resp)
	}
	if st.touchCalls != 1 {
		t.Errorf("TouchLastSeen calls = %d, want 1", st.touchCalls)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "correct-password", false)
	h := handlers.NewAuthHandlers(testLocalIssuer(t), st)
	if rec := postLogin(h, "dev@example.com", "wrong-password"); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	h := handlers.NewAuthHandlers(testLocalIssuer(t), &fakeLoginStore{creds: map[string]*store.Credentials{}})
	if rec := postLogin(h, "ghost@example.com", "whatever"); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_Disabled(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "pw-pw-pw-pw", true)
	h := handlers.NewAuthHandlers(testLocalIssuer(t), st)
	if rec := postLogin(h, "dev@example.com", "pw-pw-pw-pw"); rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for disabled account", rec.Code)
	}
}

func TestLogin_NoLocalPassword(t *testing.T) {
	// OIDC-only / invited user: row exists but has no password hash.
	st := &fakeLoginStore{creds: map[string]*store.Credentials{
		"oidc@example.com": {UserID: "u2", PasswordHash: "", Disabled: false},
	}}
	h := handlers.NewAuthHandlers(testLocalIssuer(t), st)
	if rec := postLogin(h, "oidc@example.com", "anything"); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (no local password)", rec.Code)
	}
}

func TestLogin_LocalDisabled404(t *testing.T) {
	// Nil issuer == local login disabled on this deployment.
	h := handlers.NewAuthHandlers(nil, &fakeLoginStore{creds: map[string]*store.Credentials{}})
	if rec := postLogin(h, "dev@example.com", "pw"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when local login is disabled", rec.Code)
	}
}

func TestLogin_Lockout(t *testing.T) {
	st := storeWithUser(t, "dev@example.com", "the-real-password", false)
	h := handlers.NewAuthHandlers(testLocalIssuer(t), st)

	// Five wrong attempts trip the limiter (max=5).
	for i := 0; i < 5; i++ {
		if rec := postLogin(h, "dev@example.com", "bad"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i, rec.Code)
		}
	}
	// The sixth attempt — even with the CORRECT password — is locked out.
	rec := postLogin(h, "dev@example.com", "the-real-password")
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 after lockout", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("429 response should carry a Retry-After header")
	}
}

package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// localLoginStore is the narrow store slice the local-login handler needs.
type localLoginStore interface {
	CredentialsByEmail(ctx context.Context, email string) (*store.Credentials, error)
	TouchLastSeen(ctx context.Context, id string) error
}

// AuthHandlers serves the local email+password login and the registry's own
// JWKS (ADR 0006 §3). They are only wired when local login is enabled.
type AuthHandlers struct {
	issuer  *auth.LocalIssuer
	store   localLoginStore
	lockout *loginLimiter
}

// NewAuthHandlers builds the local-auth handlers around the local token issuer
// and the credential store.
func NewAuthHandlers(issuer *auth.LocalIssuer, st localLoginStore) *AuthHandlers {
	return &AuthHandlers{
		issuer:  issuer,
		store:   st,
		lockout: newLoginLimiter(5, 15*time.Minute, 15*time.Minute),
	}
}

// Login handles POST /api/v1/auth/login: email + password → a short-lived
// registry-signed token. Failures are deliberately uniform ("invalid email or
// password") so the endpoint never reveals whether an email exists.
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	if h.issuer == nil {
		problem.Write(w, http.StatusNotFound, "not-found",
			"local login is disabled on this deployment", r.URL.Path)
		return
	}

	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if email == "" || body.Password == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"email and password are required", r.URL.Path)
		return
	}

	if retryAfter, locked := h.lockout.locked(email); locked {
		w.Header().Set("Retry-After", retryAfter)
		problem.Write(w, http.StatusTooManyRequests, "too-many-requests",
			"too many failed login attempts; try again later", r.URL.Path)
		return
	}

	creds, err := h.store.CredentialsByEmail(r.Context(), email)
	switch {
	case errors.Is(err, store.ErrNotFound):
		// No such account. Spend the same CPU as a real verification so the
		// response time can't be used to tell which emails have accounts.
		auth.VerifyPasswordDummy(body.Password)
		h.lockout.fail(email)
		unauthorizedLogin(w, r)
		return
	case err != nil:
		internalError(w, r, err)
		return
	}

	// An OIDC-only or invited account has no local password — run the same
	// dummy verification and fail uniformly with a wrong password, never
	// revealing that the account exists but can't log in locally.
	if creds.PasswordHash == "" {
		auth.VerifyPasswordDummy(body.Password)
		h.lockout.fail(email)
		unauthorizedLogin(w, r)
		return
	}

	ok, err := auth.VerifyPassword(body.Password, creds.PasswordHash)
	if err != nil || !ok {
		h.lockout.fail(email)
		unauthorizedLogin(w, r)
		return
	}

	// A disabled account is refused only after the password checks out, so a
	// caller who does not know the password cannot distinguish "disabled" from
	// "wrong password" (both 401) — closing the enumeration channel while still
	// telling the legitimate owner why they can't sign in. The disabled
	// kill-switch is also enforced at token-validation time (middleware).
	if creds.Disabled {
		problem.Write(w, http.StatusForbidden, "forbidden",
			"this account is disabled", r.URL.Path)
		return
	}

	h.lockout.reset(email)
	// Best-effort; a removed row mid-request is not a login failure.
	_ = h.store.TouchLastSeen(r.Context(), creds.UserID)

	token, err := h.issuer.Mint(creds.UserID, email)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   int(h.issuer.TTL().Seconds()),
	})
}

// JWKS serves the registry's local-token public keys so a verifier can
// self-validate locally-issued tokens, mirroring the OIDC JWKS path.
func (h *AuthHandlers) JWKS(w http.ResponseWriter, r *http.Request) {
	if h.issuer == nil {
		writeJSON(w, r, http.StatusOK, map[string]any{"keys": []any{}})
		return
	}
	writeJSON(w, r, http.StatusOK, h.issuer.JWKSJSON())
}

func unauthorizedLogin(w http.ResponseWriter, r *http.Request) {
	problem.Write(w, http.StatusUnauthorized, "unauthorized",
		"invalid email or password", r.URL.Path)
}

// loginLimiter is a small in-memory failed-attempt limiter keyed by email. It
// is best-effort (per-process, not shared across replicas) — enough to blunt
// online password guessing. Durable/distributed lockout is future work
// (ADR 0006 F4).
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attemptRecord
	max      int
	window   time.Duration
	lockFor  time.Duration
}

type attemptRecord struct {
	count       int
	firstFail   time.Time
	lockedUntil time.Time
}

func newLoginLimiter(max int, window, lockFor time.Duration) *loginLimiter {
	return &loginLimiter{
		attempts: make(map[string]*attemptRecord),
		max:      max,
		window:   window,
		lockFor:  lockFor,
	}
}

// locked reports whether email is currently locked out, and if so for how long
// (as an HTTP Retry-After seconds string).
func (l *loginLimiter) locked(email string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	rec, ok := l.attempts[email]
	if !ok {
		return "", false
	}
	if time.Now().Before(rec.lockedUntil) {
		secs := int(time.Until(rec.lockedUntil).Seconds()) + 1
		return strconv.Itoa(secs), true
	}
	return "", false
}

func (l *loginLimiter) fail(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	rec, ok := l.attempts[email]
	if !ok || now.Sub(rec.firstFail) > l.window {
		l.attempts[email] = &attemptRecord{count: 1, firstFail: now}
		return
	}
	rec.count++
	if rec.count >= l.max {
		rec.lockedUntil = now.Add(l.lockFor)
	}
}

func (l *loginLimiter) reset(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, email)
}

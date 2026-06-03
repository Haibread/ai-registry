package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// authLoginStore is the narrow store slice the local-login and refresh handlers
// need. *store.DB satisfies it.
type authLoginStore interface {
	CredentialsByEmail(ctx context.Context, email string) (*store.Credentials, error)
	GetUserByID(ctx context.Context, id string) (*store.User, error)
	TouchLastSeen(ctx context.Context, id string) error
	RevokeAllRefreshTokensForUser(ctx context.Context, userID string) (int64, error)
}

// AuthHandlers serves the local email+password login plus the token-refresh
// endpoint. Both front doors mint a registry-issued access token (Ed25519 JWT)
// returned in the response body — there is no cookie.
type AuthHandlers struct {
	tokens       *auth.TokenAuthority
	refresh      *auth.RefreshManager
	store        authLoginStore
	localEnabled bool
	lockout      *loginLimiter
}

// NewAuthHandlers builds the local-auth handlers around the token authority, the
// refresh manager, and the credential store. localEnabled mirrors
// AUTH_LOCAL_LOGIN_ENABLED.
func NewAuthHandlers(tokens *auth.TokenAuthority, refresh *auth.RefreshManager, st authLoginStore, localEnabled bool) *AuthHandlers {
	return &AuthHandlers{
		tokens:       tokens,
		refresh:      refresh,
		store:        st,
		localEnabled: localEnabled,
		lockout:      newLoginLimiter(5, 15*time.Minute, 15*time.Minute),
	}
}

// tokenPairResponse is the access + refresh token pair returned to the SPA. The
// access token goes in the Authorization header of subsequent requests; the
// refresh token is exchanged at /auth/refresh for a new pair. expiresIn is the
// access token lifetime in seconds.
type tokenPairResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int    `json:"expiresIn"`
}

// issueTokenPair mints an access token and a refresh token for a logged-in user.
func issueTokenPair(ctx context.Context, ta *auth.TokenAuthority, rm *auth.RefreshManager, mp auth.MintParams, rp auth.RefreshIssueParams) (access, refreshRaw string, expiresIn int, err error) {
	access, expiresIn, err = ta.Mint(mp)
	if err != nil {
		return "", "", 0, err
	}
	refreshRaw, err = rm.Issue(ctx, rp)
	if err != nil {
		return "", "", 0, err
	}
	return access, refreshRaw, expiresIn, nil
}

// Login handles POST /api/v1/auth/login: email + password → an access + refresh
// token pair. Failures are deliberately uniform ("invalid email or password")
// so the endpoint never reveals whether an email exists.
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	if !h.localEnabled || h.tokens == nil {
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

	// An OIDC-only or invited account has no local password — run the same dummy
	// verification and fail uniformly, never revealing that the account exists
	// but can't log in locally.
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
	// "wrong password".
	if creds.Disabled {
		problem.Write(w, http.StatusForbidden, "forbidden",
			"this account is disabled", r.URL.Path)
		return
	}

	u, err := h.store.GetUserByID(r.Context(), creds.UserID)
	if err != nil {
		internalError(w, r, err)
		return
	}

	h.lockout.reset(email)
	// Best-effort; a removed row mid-request is not a login failure.
	_ = h.store.TouchLastSeen(r.Context(), u.ID)

	access, refreshRaw, expiresIn, err := issueTokenPair(r.Context(), h.tokens, h.refresh,
		auth.MintParams{
			UserID:     u.ID,
			Email:      u.Email,
			SrvAdmin:   u.IsServerAdmin,
			AuthMethod: "local",
		},
		auth.RefreshIssueParams{
			UserID:     u.ID,
			AuthMethod: "local",
		})
	if err != nil {
		internalError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, tokenPairResponse{
		AccessToken:  access,
		RefreshToken: refreshRaw,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
	})
}

// Refresh handles POST /api/v1/auth/refresh: rotate a refresh token into a fresh
// access + refresh pair. The presented refresh token is consumed (single-use);
// replaying an already-rotated one is treated as theft and revokes the whole
// lineage. The Server-Admin flag and group snapshot are re-read here, so a
// changed is_server_admin propagates within one access-token TTL.
func (h *AuthHandlers) Refresh(w http.ResponseWriter, r *http.Request) {
	if h.tokens == nil {
		problem.Write(w, http.StatusNotFound, "not-found",
			"authentication is not configured on this deployment", r.URL.Path)
		return
	}

	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.RefreshToken == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"refreshToken is required", r.URL.Path)
		return
	}

	newRefresh, row, err := h.refresh.Rotate(r.Context(), body.RefreshToken)
	switch {
	case errors.Is(err, store.ErrRefreshReuse):
		// Theft signal: the lineage was revoked by the store. Force re-login.
		slog.WarnContext(r.Context(), "refresh token reuse detected; revoked lineage")
		unauthorizedToken(w, r)
		return
	case errors.Is(err, store.ErrNotFound):
		unauthorizedToken(w, r)
		return
	case err != nil:
		internalError(w, r, err)
		return
	}

	u, err := h.store.GetUserByID(r.Context(), row.UserID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if u.Disabled {
		// Kill-switch: a disabled account cannot refresh; drop all its tokens.
		_, _ = h.store.RevokeAllRefreshTokensForUser(r.Context(), u.ID)
		problem.Write(w, http.StatusForbidden, "forbidden",
			"this account is disabled", r.URL.Path)
		return
	}

	access, expiresIn, err := h.tokens.Mint(auth.MintParams{
		UserID:     u.ID,
		Email:      u.Email,
		Groups:     row.ClaimGroups,
		SrvAdmin:   row.ClaimAdmin || u.IsServerAdmin,
		AuthMethod: row.AuthMethod,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, tokenPairResponse{
		AccessToken:  access,
		RefreshToken: newRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
	})
}

// JWKS serves GET /.well-known/jwks.json: the registry's Ed25519 public
// verification keys, so other services can validate registry access tokens
// offline. Returns 503 when no token authority is configured.
func JWKS(ta *auth.TokenAuthority) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ta == nil {
			problem.Write(w, http.StatusServiceUnavailable, "unavailable",
				"token authority is not configured", r.URL.Path)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_ = json.NewEncoder(w).Encode(ta.JWKS())
	}
}

func unauthorizedLogin(w http.ResponseWriter, r *http.Request) {
	problem.Write(w, http.StatusUnauthorized, "unauthorized",
		"invalid email or password", r.URL.Path)
}

func unauthorizedToken(w http.ResponseWriter, r *http.Request) {
	problem.Write(w, http.StatusUnauthorized, "unauthorized",
		"invalid or expired refresh token", r.URL.Path)
}

// loginLimiter is a small in-memory failed-attempt limiter keyed by email. It is
// best-effort (per-process, not shared across replicas) — enough to blunt online
// password guessing. Durable/distributed lockout is future work.
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

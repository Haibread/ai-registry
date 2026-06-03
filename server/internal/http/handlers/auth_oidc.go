package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// oidcAuthRequestTTL bounds how long a started sign-in can sit before the
// callback must complete it.
const oidcAuthRequestTTL = 5 * time.Minute

// handoffCodeTTL bounds the window for the SPA to exchange its one-time code for
// the minted tokens after the OIDC redirect.
const handoffCodeTTL = 60 * time.Second

// oidcStore is the store slice the OIDC handlers need: federated user
// resolution + the login-transaction and handoff-code persistence that replace
// the former cookies. *store.DB satisfies it.
type oidcStore interface {
	auth.PrincipalStore
	TouchLastSeen(ctx context.Context, id string) error
	CreateOIDCAuthRequest(ctx context.Context, p store.CreateOIDCAuthRequestParams) error
	ConsumeOIDCAuthRequest(ctx context.Context, stateHash string) (nonce, codeVerifier string, err error)
	CreateHandoffCode(ctx context.Context, p store.CreateHandoffCodeParams) error
	ConsumeHandoffCode(ctx context.Context, codeHash string) (accessToken, refreshToken string, expiresIn int, err error)
}

// OIDCAuthHandlers serve the server-side OIDC broker flow. /auth/oidc/login
// redirects to the IdP, /auth/oidc/callback exchanges the code and hands the
// minted tokens to the SPA via a one-time code, /auth/oidc/exchange swaps that
// code for the tokens, and /auth/logout revokes a refresh token (and surfaces
// the IdP RP-initiated logout URL for OIDC sessions). The IdP token never
// reaches the browser.
type OIDCAuthHandlers struct {
	broker  *auth.OIDCBroker
	tokens  *auth.TokenAuthority
	refresh *auth.RefreshManager
	store   oidcStore
	// postLoginRedirect is where the browser lands after a successful login (the
	// SPA), carrying the one-time handoff code in the URL fragment. Defaults to "/".
	postLoginRedirect string
	// postLogoutRedirect is the absolute URL the IdP redirects back to after
	// RP-initiated logout. It must be on the broker client's post-logout
	// allow-list. Defaults to "/".
	postLogoutRedirect string
}

// NewOIDCAuthHandlers builds the broker handlers. A nil broker (OIDC not
// configured) makes the login/callback/exchange endpoints report 404; logout
// still revokes the registry refresh token.
func NewOIDCAuthHandlers(broker *auth.OIDCBroker, tokens *auth.TokenAuthority, refresh *auth.RefreshManager, st oidcStore, postLoginRedirect, postLogoutRedirect string) *OIDCAuthHandlers {
	if postLoginRedirect == "" {
		postLoginRedirect = "/"
	}
	if postLogoutRedirect == "" {
		postLogoutRedirect = "/"
	}
	return &OIDCAuthHandlers{
		broker:             broker,
		tokens:             tokens,
		refresh:            refresh,
		store:              st,
		postLoginRedirect:  postLoginRedirect,
		postLogoutRedirect: postLogoutRedirect,
	}
}

// Login handles GET /api/v1/auth/oidc/login: mint state/nonce/PKCE, persist the
// transaction server-side (keyed by the state hash), and 302 to the IdP.
func (h *OIDCAuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	if h.broker == nil {
		problem.Write(w, http.StatusNotFound, "not-found",
			"OIDC login is not configured on this deployment", r.URL.Path)
		return
	}
	state, err := auth.RandomToken()
	if err != nil {
		internalError(w, r, err)
		return
	}
	nonce, err := auth.RandomToken()
	if err != nil {
		internalError(w, r, err)
		return
	}
	verifier, challenge, err := auth.GeneratePKCE()
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.store.CreateOIDCAuthRequest(r.Context(), store.CreateOIDCAuthRequestParams{
		StateHash:    auth.HashToken(state),
		Nonce:        nonce,
		CodeVerifier: verifier,
		ExpiresAt:    time.Now().Add(oidcAuthRequestTTL),
	}); err != nil {
		internalError(w, r, err)
		return
	}
	http.Redirect(w, r, h.broker.AuthCodeURL(state, nonce, challenge), http.StatusFound)
}

// Callback handles GET /api/v1/auth/oidc/callback: validate state, exchange the
// code, map the identity to a users row, mint tokens, stash them under a
// one-time handoff code, and redirect the SPA with that code in the fragment.
func (h *OIDCAuthHandlers) Callback(w http.ResponseWriter, r *http.Request) {
	if h.broker == nil {
		problem.Write(w, http.StatusNotFound, "not-found",
			"OIDC login is not configured on this deployment", r.URL.Path)
		return
	}

	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		problem.Write(w, http.StatusUnauthorized, "unauthorized",
			"identity provider returned an error: "+e, r.URL.Path)
		return
	}
	state, code := q.Get("state"), q.Get("code")
	if state == "" {
		problem.Write(w, http.StatusBadRequest, "bad-request",
			"missing state; restart sign-in", r.URL.Path)
		return
	}
	if code == "" {
		problem.Write(w, http.StatusBadRequest, "bad-request",
			"missing authorization code", r.URL.Path)
		return
	}

	nonce, verifier, err := h.store.ConsumeOIDCAuthRequest(r.Context(), auth.HashToken(state))
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusBadRequest, "bad-request",
			"missing or expired login transaction; restart sign-in", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	identity, idToken, err := h.broker.Exchange(r.Context(), code, verifier, nonce)
	if err != nil {
		problem.Write(w, http.StatusUnauthorized, "unauthorized",
			"could not complete sign-in with the identity provider", r.URL.Path)
		return
	}

	u, err := auth.ResolveOrProvisionFederated(r.Context(), h.store, identity)
	if err != nil {
		problem.Write(w, http.StatusUnauthorized, "unauthorized",
			"could not resolve your account", r.URL.Path)
		return
	}
	if u.Disabled {
		problem.Write(w, http.StatusForbidden, "forbidden",
			"this account is disabled", r.URL.Path)
		return
	}
	_ = h.store.TouchLastSeen(r.Context(), u.ID)

	access, refreshRaw, expiresIn, err := issueTokenPair(r.Context(), h.tokens, h.refresh,
		auth.MintParams{
			UserID:     u.ID,
			Email:      u.Email,
			Groups:     identity.Groups,
			SrvAdmin:   identity.IsAdmin || u.IsServerAdmin,
			AuthMethod: "oidc",
		},
		auth.RefreshIssueParams{
			UserID:      u.ID,
			AuthMethod:  "oidc",
			ClaimGroups: identity.Groups,
			ClaimAdmin:  identity.IsAdmin,
			IDToken:     idToken,
		})
	if err != nil {
		internalError(w, r, err)
		return
	}

	handoff, err := auth.RandomToken()
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.store.CreateHandoffCode(r.Context(), store.CreateHandoffCodeParams{
		CodeHash:     auth.HashToken(handoff),
		AccessToken:  access,
		RefreshToken: refreshRaw,
		ExpiresIn:    expiresIn,
		ExpiresAt:    time.Now().Add(handoffCodeTTL),
	}); err != nil {
		internalError(w, r, err)
		return
	}

	// Deliver the code in the URL fragment: it never reaches the server logs /
	// Referer of the SPA, and the SPA reads it from location.hash, then POSTs it
	// to /auth/oidc/exchange.
	http.Redirect(w, r, h.postLoginRedirect+"#code="+url.QueryEscape(handoff), http.StatusFound)
}

// Exchange handles POST /api/v1/auth/oidc/exchange: swap the one-time handoff
// code for the minted access + refresh token pair (single use).
func (h *OIDCAuthHandlers) Exchange(w http.ResponseWriter, r *http.Request) {
	if h.broker == nil {
		problem.Write(w, http.StatusNotFound, "not-found",
			"OIDC login is not configured on this deployment", r.URL.Path)
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Code == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"code is required", r.URL.Path)
		return
	}
	access, refreshRaw, expiresIn, err := h.store.ConsumeHandoffCode(r.Context(), auth.HashToken(body.Code))
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusUnauthorized, "unauthorized",
			"invalid or expired code", r.URL.Path)
		return
	}
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

// logoutResponse carries the optional IdP RP-initiated logout URL the SPA should
// navigate to so the upstream SSO session ends too (OIDC sessions only).
type logoutResponse struct {
	OIDCLogoutURL string `json:"oidcLogoutUrl,omitempty"`
}

// Logout handles POST /api/v1/auth/logout: revoke the presented refresh token.
// For an OIDC session it returns the IdP's RP-initiated logout URL (with the
// stored id_token as id_token_hint) so the SPA can end the upstream SSO session.
// Idempotent — an unknown / already-revoked token still returns 200.
func (h *OIDCAuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	resp := logoutResponse{}
	if body.RefreshToken != "" {
		row, err := h.refresh.Revoke(r.Context(), body.RefreshToken)
		if err == nil && row != nil && row.AuthMethod == "oidc" && h.broker != nil {
			if u := h.broker.EndSessionURL(row.IDToken, h.postLogoutRedirect); u != "" {
				resp.OIDCLogoutURL = u
			}
		}
	}
	writeJSON(w, r, http.StatusOK, resp)
}

// compile-time assertion that *store.DB satisfies the OIDC store.
var _ oidcStore = (*store.DB)(nil)

package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// oidcTxnCookie holds the short-lived login-transaction state (state, nonce,
// PKCE verifier) between the /auth/oidc/login redirect and the /callback. It is
// HttpOnly so JS cannot read it; SameSite=Lax so it survives the top-level GET
// redirect back from the IdP.
const oidcTxnCookie = "ai_registry_oidc_txn"

// oidcCallbackStore is the store slice the OIDC callback needs: resolve / JIT
// provision the federated user, plus a last-seen touch. *store.DB satisfies it.
type oidcCallbackStore interface {
	auth.PrincipalStore
	TouchLastSeen(ctx context.Context, id string) error
}

// OIDCAuthHandlers serve the server-side OIDC broker flow (ADR 0006 amendment):
// /auth/oidc/login redirects to the IdP, /auth/oidc/callback exchanges the code
// and opens a registry session. The IdP token never reaches the browser.
type OIDCAuthHandlers struct {
	broker       *auth.OIDCBroker
	sessions     *auth.SessionManager
	store        oidcCallbackStore
	cookieSecure bool
	// postLoginRedirect is where the browser lands after a successful login
	// (the SPA). Defaults to "/".
	postLoginRedirect string
	// postLogoutRedirect is the absolute URL the IdP redirects back to after
	// RP-initiated logout (and where a local logout lands). It must be on the
	// broker client's post-logout allow-list. Defaults to "/".
	postLogoutRedirect string
}

// NewOIDCAuthHandlers builds the broker handlers. A nil broker (OIDC not
// configured) makes the login/callback endpoints report 404; logout still
// clears the registry session. postLogoutRedirect should be an absolute URL
// (the SPA origin) so the IdP can redirect back after ending its SSO session.
func NewOIDCAuthHandlers(broker *auth.OIDCBroker, sessions *auth.SessionManager, st oidcCallbackStore, cookieSecure bool, postLoginRedirect, postLogoutRedirect string) *OIDCAuthHandlers {
	if postLoginRedirect == "" {
		postLoginRedirect = "/"
	}
	if postLogoutRedirect == "" {
		postLogoutRedirect = "/"
	}
	return &OIDCAuthHandlers{
		broker:             broker,
		sessions:           sessions,
		store:              st,
		cookieSecure:       cookieSecure,
		postLoginRedirect:  postLoginRedirect,
		postLogoutRedirect: postLogoutRedirect,
	}
}

type oidcTxn struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
}

// Login handles GET /api/v1/auth/oidc/login: mint state/nonce/PKCE, stash them
// in the transaction cookie, and 302 to the IdP authorize endpoint.
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
	raw, err := json.Marshal(oidcTxn{State: state, Nonce: nonce, Verifier: verifier})
	if err != nil {
		internalError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oidcTxnCookie,
		Value:    base64.RawURLEncoding.EncodeToString(raw),
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.Redirect(w, r, h.broker.AuthCodeURL(state, nonce, challenge), http.StatusFound)
}

// Callback handles GET /api/v1/auth/oidc/callback: validate state, exchange the
// code, map the identity to a users row, open a session, and redirect the SPA.
func (h *OIDCAuthHandlers) Callback(w http.ResponseWriter, r *http.Request) {
	if h.broker == nil {
		problem.Write(w, http.StatusNotFound, "not-found",
			"OIDC login is not configured on this deployment", r.URL.Path)
		return
	}

	txn, ok := h.readTxn(r)
	h.clearTxn(w)
	if !ok {
		problem.Write(w, http.StatusBadRequest, "bad-request",
			"missing or invalid login transaction; restart sign-in", r.URL.Path)
		return
	}

	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		problem.Write(w, http.StatusUnauthorized, "unauthorized",
			"identity provider returned an error: "+e, r.URL.Path)
		return
	}
	state, code := q.Get("state"), q.Get("code")
	if state == "" || state != txn.State {
		problem.Write(w, http.StatusBadRequest, "bad-request",
			"login state mismatch; restart sign-in", r.URL.Path)
		return
	}
	if code == "" {
		problem.Write(w, http.StatusBadRequest, "bad-request",
			"missing authorization code", r.URL.Path)
		return
	}

	claims, idToken, err := h.broker.Exchange(r.Context(), code, txn.Verifier, txn.Nonce)
	if err != nil {
		problem.Write(w, http.StatusUnauthorized, "unauthorized",
			"could not complete sign-in with the identity provider", r.URL.Path)
		return
	}

	u, err := auth.ResolveOrProvisionFederated(r.Context(), h.store, claims)
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

	token, err := h.sessions.Issue(r.Context(), auth.IssueParams{
		UserID:      u.ID,
		AuthMethod:  "oidc",
		ClaimGroups: claims.Groups,
		ClaimAdmin:  claims.IsAdmin(),
		IDToken:     idToken,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}
	http.SetCookie(w, h.sessions.Cookie(token))
	http.Redirect(w, r, h.postLoginRedirect, http.StatusFound)
}

// Logout handles GET /api/v1/auth/logout for the browser SPA: it revokes the
// registry session and clears the cookie, then — for an OIDC session — bounces
// the browser through the IdP's RP-initiated logout (end_session_endpoint) so
// the IdP SSO session is terminated too, passing the stored id_token as
// id_token_hint to skip the IdP's logout-confirmation prompt. A local session
// (or a missing/expired one) just lands back at the SPA. POST /auth/logout
// remains the no-redirect variant for programmatic callers.
func (h *OIDCAuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	var authMethod, idTokenHint string
	if token := h.sessions.TokenFromRequest(r); token != "" {
		if s, err := h.sessions.Resolve(r.Context(), token); err == nil && s != nil {
			authMethod = s.AuthMethod
			idTokenHint = s.IDToken
		}
		_ = h.sessions.Revoke(r.Context(), token)
	}
	http.SetCookie(w, h.sessions.ClearCookie())

	dest := h.postLogoutRedirect
	if authMethod == "oidc" && h.broker != nil {
		if u := h.broker.EndSessionURL(idTokenHint, h.postLogoutRedirect); u != "" {
			dest = u
		}
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

func (h *OIDCAuthHandlers) readTxn(r *http.Request) (oidcTxn, bool) {
	c, err := r.Cookie(oidcTxnCookie)
	if err != nil {
		return oidcTxn{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return oidcTxn{}, false
	}
	var txn oidcTxn
	if err := json.Unmarshal(raw, &txn); err != nil || txn.State == "" {
		return oidcTxn{}, false
	}
	return txn, true
}

func (h *OIDCAuthHandlers) clearTxn(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcTxnCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// compile-time assertion that *store.DB satisfies the callback store.
var _ oidcCallbackStore = (*store.DB)(nil)

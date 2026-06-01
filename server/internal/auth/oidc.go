package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OIDCConfig is the confidential-client configuration for the brokered OIDC
// flow. The registry is a single confidential
// client; the browser never sees the IdP token.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// Scopes requested at the authorize endpoint. Defaults to
	// ["openid", "profile", "email"] when empty.
	Scopes []string
	// JWKSURLOverride lets the server fetch the id_token signing keys from an
	// internal URL when the public issuer URL is unreachable from inside the
	// network (mirrors AuthConfig.OIDCJWKSUrl). Empty uses discovery's jwks_uri.
	JWKSURLOverride string
	// InternalURL is the base URL (scheme://host) the server uses to reach the
	// IdP for back-channel calls — discovery, token exchange, and JWKS — when the
	// public Issuer is unreachable from inside the network (e.g. Keycloak in
	// Docker: the browser uses http://localhost:8080 but the server reaches it at
	// http://keycloak:8080). Only the scheme and host are used; the front-channel
	// endpoints (authorize, end_session) keep the public host the browser needs.
	// The IdP must advertise the public Issuer (pin its hostname, e.g. Keycloak's
	// KC_HOSTNAME) so `iss` and the authorize URL match. Empty disables the split
	// and every call uses the public Issuer (JWKSURLOverride still applies).
	InternalURL string
}

// oidcDiscovery is the subset of the OIDC discovery document the broker uses.
type oidcDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// OIDCBroker runs the server-side Authorization Code + PKCE flow against the
// IdP as a confidential client. It validates the returned id_token against the
// IdP JWKS and returns the claims; the token itself is never handed to the
// browser.
type OIDCBroker struct {
	cfg    OIDCConfig
	disco  oidcDiscovery
	jwks   *JWKSCache
	client *http.Client
}

// NewOIDCBroker fetches the IdP discovery document and builds the broker. It is
// constructed once at boot; a failure here means OIDC login is unavailable, but
// local login (and the rest of the server) still works.
func NewOIDCBroker(ctx context.Context, cfg OIDCConfig) (*OIDCBroker, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("oidc broker: client id and secret are required")
	}
	client := &http.Client{Timeout: 10 * time.Second}

	// Back-channel calls (discovery, token, JWKS) target InternalURL when set;
	// the front-channel endpoints (authorize, end_session) keep the public host
	// the browser must reach. See OIDCConfig.InternalURL.
	discoverySource := cfg.Issuer
	if cfg.InternalURL != "" {
		discoverySource = cfg.InternalURL
	}
	disco, err := fetchDiscovery(ctx, client, discoverySource)
	if err != nil {
		return nil, err
	}

	if cfg.InternalURL != "" {
		// The IdP must advertise the public Issuer (pin its front-channel
		// hostname) so the browser-facing authorize URL and the id_token `iss`
		// match cfg.Issuer. Otherwise discovery via the internal host would hand
		// the browser internal URLs it cannot reach — fail closed with a clear
		// message rather than redirecting users to an unreachable host.
		if strings.TrimRight(disco.Issuer, "/") != strings.TrimRight(cfg.Issuer, "/") {
			return nil, fmt.Errorf("oidc broker: IdP advertises issuer %q but OIDC_ISSUER is %q — pin the IdP front-channel hostname (e.g. Keycloak KC_HOSTNAME) so it advertises the public issuer", disco.Issuer, cfg.Issuer)
		}
		disco.TokenEndpoint = rewriteHost(disco.TokenEndpoint, cfg.InternalURL)
	}

	jwksURL := cfg.JWKSURLOverride
	if jwksURL == "" {
		jwksURL = disco.JWKSURI
		if cfg.InternalURL != "" {
			jwksURL = rewriteHost(jwksURL, cfg.InternalURL)
		}
	}
	return &OIDCBroker{
		cfg:    cfg,
		disco:  disco,
		jwks:   NewJWKSCache(jwksURL, 0),
		client: client,
	}, nil
}

// rewriteHost replaces endpoint's scheme and host with those of base, keeping
// endpoint's path and query. Used to retarget the IdP back-channel endpoints at
// an internally-reachable host. Returns endpoint unchanged if either URL fails
// to parse or base carries no host.
func rewriteHost(endpoint, base string) string {
	e, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	b, err := url.Parse(base)
	if err != nil || b.Host == "" {
		return endpoint
	}
	e.Scheme = b.Scheme
	e.Host = b.Host
	return e.String()
}

func fetchDiscovery(ctx context.Context, client *http.Client, issuer string) (oidcDiscovery, error) {
	var d oidcDiscovery
	docURL := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, docURL, nil)
	if err != nil {
		return d, fmt.Errorf("oidc discovery: building request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return d, fmt.Errorf("oidc discovery: fetching %s: %w", docURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return d, fmt.Errorf("oidc discovery: %s returned %d", docURL, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return d, fmt.Errorf("oidc discovery: decoding: %w", err)
	}
	if d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" {
		return d, fmt.Errorf("oidc discovery: missing authorization or token endpoint")
	}
	return d, nil
}

func (b *OIDCBroker) scopes() []string {
	if len(b.cfg.Scopes) == 0 {
		return []string{"openid", "profile", "email"}
	}
	return b.cfg.Scopes
}

// AuthCodeURL builds the IdP authorize URL for a login transaction. state and
// nonce are opaque random values bound to the transaction; codeChallenge is the
// S256 PKCE challenge for the matching verifier.
func (b *OIDCBroker) AuthCodeURL(state, nonce, codeChallenge string) string {
	v := url.Values{}
	v.Set("client_id", b.cfg.ClientID)
	v.Set("redirect_uri", b.cfg.RedirectURL)
	v.Set("response_type", "code")
	v.Set("scope", strings.Join(b.scopes(), " "))
	v.Set("state", state)
	v.Set("nonce", nonce)
	v.Set("code_challenge", codeChallenge)
	v.Set("code_challenge_method", "S256")
	return b.disco.AuthorizationEndpoint + "?" + v.Encode()
}

// tokenResponse is the subset of the token endpoint response we read.
type tokenResponse struct {
	IDToken          string `json:"id_token"`
	AccessToken      string `json:"access_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Exchange swaps an authorization code for tokens and returns the validated
// id_token claims plus the raw id_token (kept as a logout hint). It enforces
// PKCE (code_verifier), the issuer, the audience (== client id), expiry, and
// that the id_token nonce matches the one bound to the login transaction.
func (b *OIDCBroker) Exchange(ctx context.Context, code, codeVerifier, expectedNonce string) (*KeycloakClaims, string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", b.cfg.RedirectURL)
	form.Set("client_id", b.cfg.ClientID)
	form.Set("client_secret", b.cfg.ClientSecret)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.disco.TokenEndpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, "", fmt.Errorf("oidc exchange: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("oidc exchange: posting to token endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, "", fmt.Errorf("oidc exchange: decoding token response (status %d): %w", resp.StatusCode, err)
	}
	if tr.Error != "" {
		return nil, "", fmt.Errorf("oidc exchange: token endpoint error %q: %s", tr.Error, tr.ErrorDescription)
	}
	if resp.StatusCode != http.StatusOK || tr.IDToken == "" {
		return nil, "", fmt.Errorf("oidc exchange: token endpoint returned %d with no id_token", resp.StatusCode)
	}

	claims, err := b.verifyIDToken(ctx, tr.IDToken)
	if err != nil {
		return nil, "", err
	}
	if expectedNonce == "" || claims.Nonce != expectedNonce {
		return nil, "", fmt.Errorf("oidc exchange: id_token nonce mismatch")
	}
	return claims, tr.IDToken, nil
}

// verifyIDToken validates the id_token signature (IdP JWKS), issuer, audience
// (== client id), and expiry.
func (b *OIDCBroker) verifyIDToken(ctx context.Context, idToken string) (*KeycloakClaims, error) {
	claims := &KeycloakClaims{}
	_, err := jwt.ParseWithClaims(idToken, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		kid, _ := t.Header["kid"].(string)
		return b.jwks.GetKey(ctx, kid)
	},
		jwt.WithIssuer(b.cfg.Issuer),
		jwt.WithAudience(b.cfg.ClientID),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("oidc exchange: validating id_token: %w", err)
	}
	return claims, nil
}

// EndSessionURL returns the IdP's RP-initiated logout URL, or "" when the IdP
// does not advertise an end_session_endpoint. idTokenHint and the
// post-logout redirect are appended when present.
func (b *OIDCBroker) EndSessionURL(idTokenHint, postLogoutRedirect string) string {
	if b.disco.EndSessionEndpoint == "" {
		return ""
	}
	v := url.Values{}
	// client_id lets the IdP validate post_logout_redirect_uri against the
	// client's allow-list even when no id_token_hint is available.
	if b.cfg.ClientID != "" {
		v.Set("client_id", b.cfg.ClientID)
	}
	if idTokenHint != "" {
		v.Set("id_token_hint", idTokenHint)
	}
	if postLogoutRedirect != "" {
		v.Set("post_logout_redirect_uri", postLogoutRedirect)
	}
	if len(v) == 0 {
		return b.disco.EndSessionEndpoint
	}
	return b.disco.EndSessionEndpoint + "?" + v.Encode()
}

// GeneratePKCE returns a fresh PKCE verifier and its S256 challenge.
func GeneratePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generating pkce verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// RandomToken returns a 256-bit URL-safe random string, used for the OIDC
// state and nonce.
func RandomToken() (string, error) {
	return newToken()
}

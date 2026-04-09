// White-box tests for JWKS cache internals.
// Uses package auth so we can access unexported types (jwksKey, parseRSAPublicKey).
package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// makeRSAKey generates a test RSA key pair (1024-bit is fine for tests).
func makeRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	return priv
}

// encodeBase64URLBigInt encodes a big.Int as base64url (no padding).
func encodeBase64URLBigInt(n *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(n.Bytes())
}

// encodeBase64URLInt encodes an int as base64url (no padding).
func encodeBase64URLInt(e int) string {
	b := big.NewInt(int64(e))
	return base64.RawURLEncoding.EncodeToString(b.Bytes())
}

// newJWKSServer starts a test HTTP server that serves a JWKS with the given key/kid.
func newJWKSServer(t *testing.T, priv *rsa.PrivateKey, kid string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := jwksResponse{
			Keys: []jwksKey{
				{
					Kty: "RSA",
					Kid: kid,
					Use: "sig",
					N:   encodeBase64URLBigInt(priv.PublicKey.N),
					E:   encodeBase64URLInt(priv.PublicKey.E),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// ---------------------------------------------------------------------------
// NewJWKSCache
// ---------------------------------------------------------------------------

func TestNewJWKSCache_DefaultTTL(t *testing.T) {
	c := NewJWKSCache("http://example.com/jwks", 0)
	if c.ttl != 5*time.Minute {
		t.Errorf("ttl = %v, want 5m", c.ttl)
	}
}

func TestNewJWKSCache_CustomTTL(t *testing.T) {
	want := 10 * time.Second
	c := NewJWKSCache("http://example.com/jwks", want)
	if c.ttl != want {
		t.Errorf("ttl = %v, want %v", c.ttl, want)
	}
}

// ---------------------------------------------------------------------------
// GetKey – happy path
// ---------------------------------------------------------------------------

func TestGetKey_ReturnsFreshKey(t *testing.T) {
	priv := makeRSAKey(t)
	const kid = "key-1"
	srv := newJWKSServer(t, priv, kid)
	defer srv.Close()

	cache := NewJWKSCache(srv.URL, time.Minute)
	got, err := cache.GetKey(context.Background(), kid)
	if err != nil {
		t.Fatalf("GetKey returned error: %v", err)
	}
	if got.N.Cmp(priv.PublicKey.N) != 0 {
		t.Error("returned key modulus does not match")
	}
	if got.E != priv.PublicKey.E {
		t.Errorf("returned key exponent = %d, want %d", got.E, priv.PublicKey.E)
	}
}

func TestGetKey_CacheFastPath(t *testing.T) {
	priv := makeRSAKey(t)
	const kid = "key-fast"
	fetchCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		resp := jwksResponse{
			Keys: []jwksKey{{
				Kty: "RSA",
				Kid: kid,
				N:   encodeBase64URLBigInt(priv.PublicKey.N),
				E:   encodeBase64URLInt(priv.PublicKey.E),
			}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cache := NewJWKSCache(srv.URL, time.Minute)

	// First call triggers a fetch.
	if _, err := cache.GetKey(context.Background(), kid); err != nil {
		t.Fatalf("first GetKey: %v", err)
	}
	if fetchCount != 1 {
		t.Errorf("fetchCount after first call = %d, want 1", fetchCount)
	}

	// Second call should hit the cache without fetching again.
	if _, err := cache.GetKey(context.Background(), kid); err != nil {
		t.Fatalf("second GetKey: %v", err)
	}
	if fetchCount != 1 {
		t.Errorf("fetchCount after second call = %d, want 1 (cache hit)", fetchCount)
	}
}

// ---------------------------------------------------------------------------
// GetKey – unknown kid
// ---------------------------------------------------------------------------

func TestGetKey_UnknownKidAfterRefresh(t *testing.T) {
	priv := makeRSAKey(t)
	srv := newJWKSServer(t, priv, "existing-kid")
	defer srv.Close()

	cache := NewJWKSCache(srv.URL, time.Minute)
	_, err := cache.GetKey(context.Background(), "nonexistent-kid")
	if err == nil {
		t.Fatal("expected error for unknown kid, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetKey – stale cache triggers re-fetch
// ---------------------------------------------------------------------------

func TestGetKey_StaleCache_TriggersRefetch(t *testing.T) {
	priv := makeRSAKey(t)
	const kid = "key-stale"
	fetchCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		resp := jwksResponse{
			Keys: []jwksKey{{
				Kty: "RSA",
				Kid: kid,
				N:   encodeBase64URLBigInt(priv.PublicKey.N),
				E:   encodeBase64URLInt(priv.PublicKey.E),
			}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cache := NewJWKSCache(srv.URL, time.Minute)

	// Warm up the cache.
	if _, err := cache.GetKey(context.Background(), kid); err != nil {
		t.Fatalf("warm-up GetKey: %v", err)
	}

	// Wind back fetchAt so the cache appears stale.
	cache.mu.Lock()
	cache.fetchAt = time.Now().Add(-2 * time.Minute)
	cache.mu.Unlock()

	if _, err := cache.GetKey(context.Background(), kid); err != nil {
		t.Fatalf("stale-cache GetKey: %v", err)
	}
	if fetchCount != 2 {
		t.Errorf("fetchCount = %d, want 2 (re-fetch triggered)", fetchCount)
	}
}

// ---------------------------------------------------------------------------
// refresh – error paths
// ---------------------------------------------------------------------------

func TestRefresh_ServerDown(t *testing.T) {
	// Create and immediately close a server so the address is unreachable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	cache := NewJWKSCache(srv.URL, time.Minute)
	err := cache.refresh(context.Background())
	if err == nil {
		t.Fatal("expected error when server is down, got nil")
	}
}

func TestRefresh_Non200Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cache := NewJWKSCache(srv.URL, time.Minute)
	err := cache.refresh(context.Background())
	if err == nil {
		t.Fatal("expected error on non-200 response, got nil")
	}
}

func TestRefresh_SkipsNonRSAKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := jwksResponse{
			Keys: []jwksKey{
				{Kty: "EC", Kid: "ec-key"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cache := NewJWKSCache(srv.URL, time.Minute)
	if err := cache.refresh(context.Background()); err != nil {
		t.Fatalf("refresh returned unexpected error: %v", err)
	}

	cache.mu.RLock()
	count := len(cache.keys)
	cache.mu.RUnlock()
	if count != 0 {
		t.Errorf("keys count = %d, want 0 (EC key should be skipped)", count)
	}
}

func TestRefresh_MalformedKeySkipped(t *testing.T) {
	priv := makeRSAKey(t)
	const goodKid = "good-key"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := jwksResponse{
			Keys: []jwksKey{
				// Malformed RSA key (bad N/E).
				{Kty: "RSA", Kid: "bad-key", N: "!!!invalid base64", E: "!!!invalid"},
				// Valid RSA key.
				{
					Kty: "RSA",
					Kid: goodKid,
					N:   encodeBase64URLBigInt(priv.PublicKey.N),
					E:   encodeBase64URLInt(priv.PublicKey.E),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cache := NewJWKSCache(srv.URL, time.Minute)
	if err := cache.refresh(context.Background()); err != nil {
		t.Fatalf("refresh returned unexpected error: %v", err)
	}

	cache.mu.RLock()
	_, hasBad := cache.keys["bad-key"]
	_, hasGood := cache.keys[goodKid]
	cache.mu.RUnlock()

	if hasBad {
		t.Error("malformed key should have been skipped")
	}
	if !hasGood {
		t.Error("valid key should have been loaded")
	}
}

// ---------------------------------------------------------------------------
// parseRSAPublicKey
// ---------------------------------------------------------------------------

func TestParseRSAPublicKey_Success(t *testing.T) {
	priv := makeRSAKey(t)
	k := jwksKey{
		Kty: "RSA",
		Kid: "k1",
		N:   encodeBase64URLBigInt(priv.PublicKey.N),
		E:   encodeBase64URLInt(priv.PublicKey.E),
	}
	got, err := parseRSAPublicKey(k)
	if err != nil {
		t.Fatalf("parseRSAPublicKey error: %v", err)
	}
	if got.N.Cmp(priv.PublicKey.N) != 0 {
		t.Error("N mismatch")
	}
	if got.E != priv.PublicKey.E {
		t.Errorf("E = %d, want %d", got.E, priv.PublicKey.E)
	}
}

func TestParseRSAPublicKey_InvalidN(t *testing.T) {
	priv := makeRSAKey(t)
	k := jwksKey{
		Kty: "RSA",
		Kid: "bad-n",
		N:   "not-valid-base64url!!!", // invalid base64url
		E:   encodeBase64URLInt(priv.PublicKey.E),
	}
	_, err := parseRSAPublicKey(k)
	if err == nil {
		t.Fatal("expected error for invalid N, got nil")
	}
}

func TestParseRSAPublicKey_InvalidE(t *testing.T) {
	priv := makeRSAKey(t)
	k := jwksKey{
		Kty: "RSA",
		Kid: "bad-e",
		N:   encodeBase64URLBigInt(priv.PublicKey.N),
		E:   "not-valid-base64url!!!", // invalid base64url
	}
	_, err := parseRSAPublicKey(k)
	if err == nil {
		t.Fatal("expected error for invalid E, got nil")
	}
}

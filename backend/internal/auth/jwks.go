package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// jwksKey is one entry in the JSON Web Key Set.
type jwksKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

// JWKSCache fetches and caches the JWKS from a remote endpoint, refreshing
// automatically after TTL or when an unknown kid is encountered.
type JWKSCache struct {
	endpoint string
	client   *http.Client
	ttl      time.Duration

	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	fetchAt time.Time
}

// NewJWKSCache creates a JWKSCache that will fetch from endpoint and
// refresh every ttl. A zero ttl defaults to 5 minutes.
func NewJWKSCache(endpoint string, ttl time.Duration) *JWKSCache {
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	return &JWKSCache{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
		ttl:      ttl,
		keys:     make(map[string]*rsa.PublicKey),
	}
}

// GetKey returns the RSA public key for the given kid.
// It refreshes the key set if the cache is stale or the kid is unknown.
func (c *JWKSCache) GetKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Fast path: key in cache and still fresh.
	c.mu.RLock()
	key, ok := c.keys[kid]
	fresh := time.Since(c.fetchAt) < c.ttl
	c.mu.RUnlock()

	if ok && fresh {
		return key, nil
	}

	// Slow path: refresh.
	if err := c.refresh(ctx); err != nil {
		return nil, err
	}

	c.mu.RLock()
	key, ok = c.keys[kid]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown key id %q", kid)
	}
	return key, nil
}

func (c *JWKSCache) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return fmt.Errorf("building JWKS request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching JWKS from %s: %w", c.endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decoding JWKS: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, parseErr := parseRSAPublicKey(k)
		if parseErr != nil {
			continue // skip malformed keys
		}
		newKeys[k.Kid] = pub
	}

	c.mu.Lock()
	c.keys = newKeys
	c.fetchAt = time.Now()
	c.mu.Unlock()
	return nil
}

func parseRSAPublicKey(k jwksKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decoding RSA modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decoding RSA exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	var eBig big.Int
	eBig.SetBytes(eBytes)
	e := int(eBig.Int64())

	return &rsa.PublicKey{N: n, E: e}, nil
}

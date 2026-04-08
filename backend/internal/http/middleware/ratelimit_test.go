package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/http/middleware"
)

func TestRateLimit_WithinLimit(t *testing.T) {
	handler := middleware.RateLimit(5, time.Minute)(okHandler())

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:5678"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i+1, rec.Code)
		}
	}
}

func TestRateLimit_ExceedsLimit(t *testing.T) {
	handler := middleware.RateLimit(3, time.Minute)(okHandler())

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:5678"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i+1, rec.Code)
		}
	}

	// 4th request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("over-limit request: status = %d, want 429", rec.Code)
	}
}

func TestRateLimit_WindowReset(t *testing.T) {
	// Use a very short window to test reset behaviour
	window := 50 * time.Millisecond
	handler := middleware.RateLimit(2, window)(okHandler())

	// Use up the limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:5678"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Should be rate limited now
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after limit, got %d", rec.Code)
	}

	// Wait for window to expire
	time.Sleep(window + 10*time.Millisecond)

	// Should be allowed again
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("after window reset: status = %d, want 200", rec.Code)
	}
}

func TestRateLimit_XForwardedFor(t *testing.T) {
	handler := middleware.RateLimit(2, time.Minute)(okHandler())

	// Use up the limit for the XFF IP
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		req.RemoteAddr = "1.2.3.4:5678"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// XFF IP should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("XFF IP rate limit: status = %d, want 429", rec.Code)
	}

	// RemoteAddr (different IP) should still be allowed
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:5678" // no XFF header
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("RemoteAddr IP (different from XFF): status = %d, want 200", rec.Code)
	}
}

func TestRateLimit_DifferentIPsIndependent(t *testing.T) {
	handler := middleware.RateLimit(2, time.Minute)(okHandler())

	// IP1 uses up its limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.1.1.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// IP1 should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.1.1.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("IP1 rate limit: status = %d, want 429", rec.Code)
	}

	// IP2 should not be affected
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "2.2.2.2:5678"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("IP2 (independent): status = %d, want 200", rec.Code)
	}
}

func TestRateLimit_RetryAfterHeader(t *testing.T) {
	handler := middleware.RateLimit(1, time.Minute)(okHandler())

	// Use up limit
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Exceed limit
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Error("expected Retry-After header on 429 response")
	}
}

package middleware_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/http/middleware"
)

func TestClientIP(t *testing.T) {
	_, v4Proxy, _ := net.ParseCIDR("10.0.0.0/8")
	_, v6Proxy, _ := net.ParseCIDR("fd00::/8")

	tests := []struct {
		name    string
		trusted *net.IPNet
		remote  string
		xff     []string
		want    string
	}{
		{"no trusted cidr ignores xff", nil, "203.0.113.5:4444", []string{"198.51.100.1"}, "203.0.113.5"},
		{"untrusted peer ignores xff", v4Proxy, "203.0.113.5:4444", []string{"198.51.100.1"}, "203.0.113.5"},
		{"trusted peer without xff", v4Proxy, "10.0.0.2:4444", nil, "10.0.0.2"},
		{"single hop", v4Proxy, "10.0.0.2:4444", []string{"198.51.100.1"}, "198.51.100.1"},
		{"spoofed leftmost entry ignored", v4Proxy, "10.0.0.2:4444", []string{"1.1.1.1, 198.51.100.1"}, "198.51.100.1"},
		{"chain of trusted proxies", v4Proxy, "10.0.0.2:4444", []string{"1.1.1.1, 198.51.100.1, 10.1.1.1, 10.2.2.2"}, "198.51.100.1"},
		{"multiple xff headers", v4Proxy, "10.0.0.2:4444", []string{"1.1.1.1", "198.51.100.1, 10.1.1.1"}, "198.51.100.1"},
		{"whitespace and ports", v4Proxy, "10.0.0.2:4444", []string{"  1.1.1.1 ,198.51.100.1:5555 ,  10.1.1.1  "}, "198.51.100.1"},
		{"all hops trusted returns leftmost", v4Proxy, "10.0.0.2:4444", []string{"10.9.9.9, 10.1.1.1"}, "10.9.9.9"},
		{"garbage left of client is ignored", v4Proxy, "10.0.0.2:4444", []string{"not-an-ip, 198.51.100.1"}, "198.51.100.1"},
		{"garbage in trusted chain stops the walk", v4Proxy, "10.0.0.2:4444", []string{"1.1.1.1, bogus, 10.1.1.1"}, "10.1.1.1"},
		{"garbage right after peer falls back to peer", v4Proxy, "10.0.0.2:4444", []string{"1.1.1.1, ???"}, "10.0.0.2"},
		{"empty entries are garbage", v4Proxy, "10.0.0.2:4444", []string{"1.1.1.1,"}, "10.0.0.2"},
		{"ipv6 client behind ipv6 proxy", v6Proxy, "[fd00::1]:4444", []string{"2001:db8::bad, 2001:db8::1, fd00::2"}, "2001:db8::1"},
		{"bracketed ipv6 with port", v6Proxy, "[fd00::1]:4444", []string{"[2001:db8::1]:5555"}, "2001:db8::1"},
		{"ipv6 is canonicalised", v6Proxy, "[fd00::1]:4444", []string{"2001:0db8:0:0:0:0:0:0001"}, "2001:db8::1"},
		{"untrusted ipv6 peer ignores xff", v4Proxy, "[2001:db8::9]:4444", []string{"198.51.100.1"}, "2001:db8::9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			for _, v := range tt.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			if got := middleware.ClientIP(req, tt.trusted); got != tt.want {
				t.Errorf("ClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimit_RotatingSpoofedXFFDoesNotBypassLimit(t *testing.T) {
	_, trusted, _ := net.ParseCIDR("10.0.0.0/8")
	handler := middleware.RateLimit(2, time.Minute, nil, trusted)(okHandler())

	for i, spoof := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.2:4444"
		req.Header.Set("X-Forwarded-For", spoof+", 198.51.100.1")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		want := http.StatusOK
		if i == 2 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Errorf("request %d (spoof %s): status = %d, want %d", i+1, spoof, rec.Code, want)
		}
	}
}

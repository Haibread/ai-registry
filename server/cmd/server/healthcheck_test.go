package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthcheckURL(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{name: "port only", addr: ":8081", want: "http://127.0.0.1:8081/healthz"},
		{name: "ipv4 wildcard", addr: "0.0.0.0:8080", want: "http://127.0.0.1:8080/healthz"},
		{name: "ipv6 wildcard", addr: "[::]:8080", want: "http://127.0.0.1:8080/healthz"},
		{name: "explicit host", addr: "10.0.0.5:9000", want: "http://10.0.0.5:9000/healthz"},
		{name: "explicit ipv6 host", addr: "[::1]:9000", want: "http://[::1]:9000/healthz"},
		{name: "missing port", addr: "localhost", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := healthcheckURL(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("healthcheckURL(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("healthcheckURL(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestProbeHealth(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "healthy", status: http.StatusOK},
		{name: "unhealthy", status: http.StatusServiceUnavailable, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/healthz" {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			err := probeHealth(context.Background(), srv.Client(), srv.URL+"/healthz")
			if (err != nil) != tt.wantErr {
				t.Fatalf("probeHealth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProbeHealthUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL + "/healthz"
	srv.Close()

	if err := probeHealth(context.Background(), http.DefaultClient, url); err == nil {
		t.Fatal("probeHealth() on a closed server returned nil error")
	}
}

package http_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	stdhttp "github.com/haibread/ai-registry/internal/http"
)

func startDrainTestServer(t *testing.T, draining *atomic.Bool) (*stdhttp.Server, string) {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if draining.Load() {
			_, _ = io.WriteString(w, "draining")
			return
		}
		_, _ = io.WriteString(w, "ready")
	})
	srv := stdhttp.NewServer(handler, stdhttp.ServerConfig{})
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(l) }()
	return srv, "http://" + l.Addr().String()
}

func get(url string) (string, error) {
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

func TestDrainAndShutdown_FailsReadinessThenStopsAfterDelay(t *testing.T) {
	var draining atomic.Bool
	srv, url := startDrainTestServer(t, &draining)
	if body, err := get(url); err != nil || body != "ready" {
		t.Fatalf("before shutdown: body=%q err=%v", body, err)
	}

	const delay = 300 * time.Millisecond
	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- srv.DrainAndShutdown(context.Background(), &draining, delay, 5*time.Second) }()

	// Readiness flips at once, while the listener still accepts requests.
	deadline := time.Now().Add(delay / 2)
	for !draining.Load() {
		if time.Now().After(deadline) {
			t.Fatal("draining flag not set promptly")
		}
		time.Sleep(time.Millisecond)
	}
	if body, err := get(url); err != nil || body != "draining" {
		t.Fatalf("during drain delay: body=%q err=%v, want the server still serving", body, err)
	}

	if err := <-done; err != nil {
		t.Fatalf("DrainAndShutdown: %v", err)
	}
	if elapsed := time.Since(start); elapsed < delay {
		t.Errorf("shutdown after %v, want at least the %v drain delay", elapsed, delay)
	}
	if _, err := get(url); err == nil {
		t.Error("server still accepts connections after shutdown")
	}
}

func TestDrainAndShutdown_CancelledContextSkipsDelay(t *testing.T) {
	var draining atomic.Bool
	srv, _ := startDrainTestServer(t, &draining)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if err := srv.DrainAndShutdown(ctx, &draining, time.Hour, 5*time.Second); err != nil {
		t.Fatalf("DrainAndShutdown: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %v; a cancelled context must cut the drain delay short", elapsed)
	}
	if !draining.Load() {
		t.Error("draining flag not set")
	}
}

func TestDrainAndShutdown_ZeroDelay(t *testing.T) {
	var draining atomic.Bool
	srv, url := startDrainTestServer(t, &draining)

	if err := srv.DrainAndShutdown(context.Background(), &draining, 0, 5*time.Second); err != nil {
		t.Fatalf("DrainAndShutdown: %v", err)
	}
	if _, err := get(url); err == nil {
		t.Error("server still accepts connections after shutdown")
	}
}

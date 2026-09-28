package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type fakeSweepStore struct {
	mu         sync.Mutex
	calls      map[string]int
	refreshErr error
	swept      chan struct{}
}

func newFakeSweepStore() *fakeSweepStore {
	return &fakeSweepStore{calls: map[string]int{}, swept: make(chan struct{}, 100)}
}

func (f *fakeSweepStore) record(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[name]++
}

func (f *fakeSweepStore) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[name]
}

func (f *fakeSweepStore) DeleteExpiredRefreshTokens(context.Context) (int64, error) {
	f.record("refresh")
	return 1, f.refreshErr
}

func (f *fakeSweepStore) DeleteExpiredOIDCAuthRequests(context.Context) (int64, error) {
	f.record("oidc")
	return 2, nil
}

func (f *fakeSweepStore) DeleteExpiredHandoffCodes(context.Context) (int64, error) {
	f.record("handoff")
	f.swept <- struct{}{}
	return 3, nil
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestSweep_ErrorDoesNotStopOtherTables(t *testing.T) {
	s := newFakeSweepStore()
	s.refreshErr = errors.New("boom")

	Sweep(context.Background(), s, discardLogger())

	for _, name := range []string{"refresh", "oidc", "handoff"} {
		if got := s.count(name); got != 1 {
			t.Errorf("%s swept %d times, want 1", name, got)
		}
	}
}

func TestRunSweeper_TicksAndStopsOnCancel(t *testing.T) {
	s := newFakeSweepStore()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		RunSweeper(ctx, s, 5*time.Millisecond, discardLogger())
	}()

	for i := 0; i < 3; i++ {
		select {
		case <-s.swept:
		case <-time.After(5 * time.Second):
			t.Fatalf("sweep %d did not run", i+1)
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunSweeper did not return after cancel")
	}

	after := s.count("handoff")
	time.Sleep(20 * time.Millisecond)
	if got := s.count("handoff"); got != after {
		t.Errorf("sweeps continued after cancel: %d -> %d", after, got)
	}
}

func TestSweep_CancelledContextSkipsWork(t *testing.T) {
	s := newFakeSweepStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	Sweep(ctx, s, discardLogger())

	if got := s.count("refresh"); got != 0 {
		t.Errorf("refresh swept %d times on a cancelled context, want 0", got)
	}
}

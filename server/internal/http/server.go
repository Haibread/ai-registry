package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// Server wraps net/http.Server with graceful shutdown support.
type Server struct {
	srv *http.Server
}

// ServerConfig holds per-server settings.
type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// NewServer creates a Server with the given handler and configuration.
func NewServer(handler http.Handler, cfg ServerConfig) *Server {
	return &Server{
		srv: &http.Server{
			Addr:         cfg.Addr,
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
	}
}

// ListenAndServe starts the HTTP server. It blocks until the server is closed.
// http.ErrServerClosed is suppressed and treated as a clean shutdown.
func (s *Server) ListenAndServe() error {
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server error: %w", err)
	}
	return nil
}

// Serve accepts connections on l until the server is shut down.
// http.ErrServerClosed is suppressed and treated as a clean shutdown.
func (s *Server) Serve(l net.Listener) error {
	if err := s.srv.Serve(l); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server error: %w", err)
	}
	return nil
}

// Shutdown gracefully drains active connections. It respects the context
// deadline for the drain timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// DrainAndShutdown sets draining so /readyz fails, keeps serving for
// drainDelay so load balancers stop routing here, then shuts down, letting
// in-flight requests finish within timeout. A done ctx cuts the delay short.
func (s *Server) DrainAndShutdown(ctx context.Context, draining *atomic.Bool, drainDelay, timeout time.Duration) error {
	draining.Store(true)
	if drainDelay > 0 {
		t := time.NewTimer(drainDelay)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.Shutdown(shutdownCtx)
}

// Addr returns the configured listen address.
func (s *Server) Addr() string {
	return s.srv.Addr
}

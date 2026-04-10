package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	authpkg "github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/bootstrap"
	"github.com/haibread/ai-registry/internal/config"
	registryhttp "github.com/haibread/ai-registry/internal/http"
	"github.com/haibread/ai-registry/internal/observability"
	"github.com/haibread/ai-registry/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	// ── Flags ────────────────────────────────────────────────────────────────
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	configFile := fs.String("config", "", "path to YAML config file (overrides CONFIG_FILE env var)")
	bootstrapFile := fs.String("bootstrap-file", "", "path to YAML/JSON bootstrap file; loads initial data then starts the server (overrides BOOTSTRAP_FILE env var)")
	// Parse only known flags; ignore unrecognised ones so that test harnesses
	// can inject extra arguments without breaking the server.
	_ = fs.Parse(os.Args[1:])

	// Env var fallback for bootstrap file (consistent with other config values).
	if *bootstrapFile == "" {
		*bootstrapFile = os.Getenv("BOOTSTRAP_FILE")
	}

	// ── Config ───────────────────────────────────────────────────────────────
	cfg, err := config.Load(*configFile)
	if err != nil {
		return err
	}

	// ── Logger ───────────────────────────────────────────────────────────────
	logger := observability.NewLogger(cfg.Log.Level)
	slog.SetDefault(logger)

	// ── OTel ─────────────────────────────────────────────────────────────────
	ctx := context.Background()
	otelShutdown, err := observability.Setup(ctx, observability.Config{
		ServiceName:    cfg.OTel.ServiceName,
		ServiceVersion: cfg.OTel.ServiceVersion,
		OTLPEndpoint:   cfg.OTel.OTLPEndpoint,
		LogLevel:       cfg.Log.Level,
	})
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := otelShutdown(shutdownCtx); err != nil {
			logger.Error("otel shutdown error", slog.String("error", err.Error()))
		}
	}()

	// ── Metrics ──────────────────────────────────────────────────────────────
	metrics, err := observability.InitMetrics()
	if err != nil {
		return err
	}

	// ── Database ─────────────────────────────────────────────────────────────
	logger.Info("connecting to database")
	db, err := store.Open(ctx, cfg.Database.URL, cfg.Database.MaxConns, cfg.Database.MinConns)
	if err != nil {
		return err
	}
	defer db.Close()
	logger.Info("database connected")

	// ── Migrations ───────────────────────────────────────────────────────────
	logger.Info("running migrations")
	if err := store.Migrate(cfg.Database.URL); err != nil {
		return err
	}
	logger.Info("migrations complete")

	// ── Bootstrap (optional) ─────────────────────────────────────────────────
	if *bootstrapFile != "" {
		logger.Info("loading bootstrap file", slog.String("path", *bootstrapFile))
		spec, err := bootstrap.LoadSpec(*bootstrapFile)
		if err != nil {
			return err
		}
		if err := bootstrap.Run(ctx, db, spec, logger); err != nil {
			return err
		}
	}

	// ── Trusted proxy ─────────────────────────────────────────────────────────
	var trustedProxy *net.IPNet
	if cidr := cfg.HTTP.TrustedProxyCIDR; cidr != "" {
		_, trustedProxy, err = net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("invalid TRUSTED_PROXY_CIDR %q: %w", cidr, err)
		}
		logger.Info("trusted proxy configured", slog.String("cidr", cidr))
	}

	// ── HTTP server ──────────────────────────────────────────────────────────
	handler := registryhttp.NewRouter(registryhttp.RouterDeps{
		Logger:  logger,
		DB:      db,
		Metrics: metrics,
		AuthConf: authpkg.Config{
			OIDCIssuer:  cfg.Auth.OIDCIssuer,
			OIDCJWKSUrl: cfg.Auth.OIDCJWKSUrl,
		},
		CORSOrigins:  cfg.HTTP.CORSOrigins,
		TrustedProxy: trustedProxy,
	})
	srv := registryhttp.NewServer(handler, registryhttp.ServerConfig{
		Addr:         cfg.HTTP.Addr,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	})

	// ── Graceful shutdown ────────────────────────────────────────────────────
	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", slog.String("addr", srv.Addr()))
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case sig := <-quit:
		logger.Info("received signal, shutting down", slog.String("signal", sig.String()))
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}

	return nil
}

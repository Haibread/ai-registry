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
	"strings"
	"syscall"
	"time"

	authpkg "github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/bootstrap"
	"github.com/haibread/ai-registry/internal/config"
	registryhttp "github.com/haibread/ai-registry/internal/http"
	"github.com/haibread/ai-registry/internal/observability"
	"github.com/haibread/ai-registry/internal/store"
)

// Build metadata, injected at link time via -ldflags "-X main.Version=… …".
// Defaults make local `go run` / `go build` produce a legible identity without
// requiring the ldflags dance. The publish workflow passes real values.
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
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
	bootstrapFile := fs.String("bootstrap-file", "", "path to YAML/JSON bootstrap file; loads initial data then starts the server (overrides config layer's BOOTSTRAP_FILE / bootstrap_file)")
	// Parse only known flags; tolerate unrecognised ones so that test
	// harnesses (e.g. `go test`) can inject extra arguments without
	// breaking the server. ContinueOnError already writes the error to
	// stderr; we additionally surface it via a structured warning so log
	// aggregators that don't capture raw stderr still see it. flag.ErrHelp
	// is the user-requested-help case — exit cleanly without an error log.
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		slog.Warn("server: ignoring flag parse error and continuing with defaults",
			slog.String("error", err.Error()),
		)
	}

	// ── Config ───────────────────────────────────────────────────────────────
	cfg, err := config.Load(*configFile)
	if err != nil {
		return err
	}

	// Resolve bootstrap file: --bootstrap-file flag wins over the config
	// layer's resolved value (env > YAML > default).
	bootstrapPath := *bootstrapFile
	if bootstrapPath == "" {
		bootstrapPath = cfg.BootstrapFile
	}

	// ── Logger ───────────────────────────────────────────────────────────────
	logger := observability.NewLogger(cfg.Log.Level)
	slog.SetDefault(logger)
	logger.Info("starting ai-registry server",
		slog.String("version", Version),
		slog.String("commit", Commit),
		slog.String("built", BuildTime),
	)

	// ── OTel ─────────────────────────────────────────────────────────────────
	// Prefer the ldflags-injected Version over the config default; operators
	// can still override with OTEL_SERVICE_VERSION.
	serviceVersion := cfg.OTel.ServiceVersion
	if os.Getenv("OTEL_SERVICE_VERSION") == "" && Version != "dev" {
		serviceVersion = Version
	}
	ctx := context.Background()
	otelShutdown, err := observability.Setup(ctx, observability.Config{
		ServiceName:    cfg.OTel.ServiceName,
		ServiceVersion: serviceVersion,
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

	// ── RBAC boot seed ────────────────────────────────────────────
	// Idempotent: ensures the reviewer group + its global Reviewer grant, and
	// seeds the bootstrap Server Admin (create-only). Runs after migrations so
	// the RBAC tables exist.
	var bootstrapAdminHash string
	if cfg.Auth.BootstrapAdminEmail != "" {
		bootstrapAdminHash, err = authpkg.HashPassword(cfg.Auth.BootstrapAdminPassword)
		if err != nil {
			return fmt.Errorf("hashing bootstrap admin password: %w", err)
		}
	}
	if err := db.SeedRBAC(ctx, store.SeedRBACParams{
		ReviewerGroupSlug:          cfg.Auth.ReviewerGroup,
		BootstrapAdminEmail:        cfg.Auth.BootstrapAdminEmail,
		BootstrapAdminPasswordHash: bootstrapAdminHash,
	}); err != nil {
		return err
	}
	logger.Info("rbac seed complete")

	// ── Bootstrap (optional) ─────────────────────────────────────────────────
	if bootstrapPath != "" {
		logger.Info("loading bootstrap file", slog.String("path", bootstrapPath))
		spec, err := bootstrap.LoadSpec(bootstrapPath)
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

	// ── Sessions ────────────────────────────
	// Both front doors end in a registry session behind an HttpOnly cookie.
	sessions := authpkg.NewSessionManager(db, authpkg.SessionConfig{
		CookieName: cfg.Auth.SessionCookieName,
		TTL:        cfg.Auth.SessionTTL,
		Secure:     cfg.Auth.SessionCookieSecure,
		SameSite:   authpkg.ParseSameSite(cfg.Auth.SessionCookieSameSite),
	})
	if cfg.Auth.LocalLoginEnabled {
		logger.Info("local login enabled")
	}

	// ── OIDC broker (optional) ────────────────────────────────────────────────
	// Built only when a confidential client is configured. OIDC is brokered
	// server-side; the IdP token never reaches the browser. A discovery failure
	// here is fatal (operators asked for OIDC), but local login is independent.
	var oidcBroker *authpkg.OIDCBroker
	oidcEnabled := false
	if cfg.Auth.OIDCClientID != "" && cfg.Auth.OIDCClientSecret != "" {
		redirect := cfg.Auth.OIDCRedirectURL
		if redirect == "" && cfg.HTTP.PublicBaseURL != "" {
			redirect = strings.TrimRight(cfg.HTTP.PublicBaseURL, "/") + "/api/v1/auth/oidc/callback"
		}
		oidcBroker, err = authpkg.NewOIDCBroker(ctx, authpkg.OIDCConfig{
			Issuer:          cfg.Auth.OIDCIssuer,
			ClientID:        cfg.Auth.OIDCClientID,
			ClientSecret:    cfg.Auth.OIDCClientSecret,
			RedirectURL:     redirect,
			Scopes:          cfg.Auth.OIDCScopes,
			JWKSURLOverride: cfg.Auth.OIDCJWKSUrl,
			InternalURL:     cfg.Auth.OIDCInternalURL,
		})
		if err != nil {
			return fmt.Errorf("initialising OIDC broker: %w", err)
		}
		oidcEnabled = true
		logger.Info("oidc broker enabled", slog.String("redirect_url", redirect))
	}

	// ── HTTP server ──────────────────────────────────────────────────────────
	handler := registryhttp.NewRouter(registryhttp.RouterDeps{
		Logger:             logger,
		DB:                 db,
		Metrics:            metrics,
		CORSOrigins:        cfg.HTTP.CORSOrigins,
		TrustedProxy:       trustedProxy,
		PublicRateLimitRPM: cfg.HTTP.PublicRateLimitRPM,
		PublicBaseURL:      cfg.HTTP.PublicBaseURL,
		Sessions:           sessions,
		OIDC:               oidcBroker,
		LocalLoginEnabled:  cfg.Auth.LocalLoginEnabled,
		OIDCEnabled:        oidcEnabled,
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

package auth

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// SweepStore is the store slice the auth-state sweep needs. *store.DB
// satisfies it.
type SweepStore interface {
	DeleteExpiredRefreshTokens(ctx context.Context) (int64, error)
	DeleteExpiredOIDCAuthRequests(ctx context.Context) (int64, error)
	DeleteExpiredHandoffCodes(ctx context.Context) (int64, error)
}

// RunSweeper purges expired auth state once at start and then every interval,
// until ctx is cancelled. Failures are logged and retried on the next tick.
// Every statement is a plain DELETE, so replicas sweeping concurrently are
// harmless.
func RunSweeper(ctx context.Context, s SweepStore, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		Sweep(ctx, s, logger)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Sweep runs one pass over the auth-state tables.
func Sweep(ctx context.Context, s SweepStore, logger *slog.Logger) {
	ctx, span := otel.GetTracerProvider().Tracer("ai-registry/auth").Start(ctx, "auth.Sweep")
	defer span.End()

	for _, step := range []struct {
		table string
		run   func(context.Context) (int64, error)
	}{
		{"refresh_tokens", s.DeleteExpiredRefreshTokens},
		{"oidc_auth_requests", s.DeleteExpiredOIDCAuthRequests},
		{"auth_handoff_codes", s.DeleteExpiredHandoffCodes},
	} {
		if ctx.Err() != nil {
			return
		}
		n, err := step.run(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			logger.WarnContext(ctx, "auth sweep failed",
				slog.String("table", step.table), slog.String("error", err.Error()))
			continue
		}
		logger.DebugContext(ctx, "auth sweep",
			slog.String("table", step.table), slog.Int64("deleted", n))
	}
}

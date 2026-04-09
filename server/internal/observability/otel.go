// Package observability initialises OpenTelemetry providers (tracer, meter)
// and the structured logger. Call Setup once at startup; call the returned
// shutdown function on graceful termination.
package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	promexporter "go.opentelemetry.io/otel/exporters/prometheus"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds the subset of application config needed by this package.
type Config struct {
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string
	LogLevel       string
}

// Setup initialises the global OTel TracerProvider and MeterProvider, and
// returns a shutdown function that must be called before the process exits.
func Setup(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTel resource: %w", err)
	}

	var shutdownFns []func(context.Context) error

	// ── Trace provider ───────────────────────────────────────────────────────
	tpOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	}

	if cfg.OTLPEndpoint != "" {
		conn, dialErr := grpc.NewClient(cfg.OTLPEndpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if dialErr != nil {
			return nil, fmt.Errorf("dialing OTel collector at %s: %w", cfg.OTLPEndpoint, dialErr)
		}
		traceExp, expErr := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
		if expErr != nil {
			return nil, fmt.Errorf("creating OTLP trace exporter: %w", expErr)
		}
		tpOpts = append(tpOpts, sdktrace.WithBatcher(traceExp))
	}

	tp := sdktrace.NewTracerProvider(tpOpts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	shutdownFns = append(shutdownFns, tp.Shutdown)

	// ── Metric provider (Prometheus pull-based) ───────────────────────────────
	promExp, err := promexporter.New()
	if err != nil {
		return nil, fmt.Errorf("creating Prometheus exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(promExp),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)
	shutdownFns = append(shutdownFns, mp.Shutdown)

	return func(ctx context.Context) error {
		var errs []error
		for _, fn := range shutdownFns {
			if fnErr := fn(ctx); fnErr != nil {
				errs = append(errs, fnErr)
			}
		}
		return errors.Join(errs...)
	}, nil
}

// NewLogger returns a structured JSON slog.Logger that injects trace_id and
// span_id fields from the active span in the context on each log record.
func NewLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := &traceHandler{
		inner: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}),
	}
	return slog.New(h)
}

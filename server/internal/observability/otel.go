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

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
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

	// ── Shared OTLP connection ────────────────────────────────────────────────
	// A single gRPC connection to the collector is shared by the trace, metric,
	// and log exporters when an endpoint is configured (the CLAUDE.md mandate is
	// "all signals via OTLP"). Nil when no endpoint is set.
	var conn *grpc.ClientConn
	if cfg.OTLPEndpoint != "" {
		c, dialErr := grpc.NewClient(cfg.OTLPEndpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if dialErr != nil {
			return nil, fmt.Errorf("dialing OTel collector at %s: %w", cfg.OTLPEndpoint, dialErr)
		}
		conn = c
	}

	// ── Trace provider ───────────────────────────────────────────────────────
	tpOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	}
	if conn != nil {
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

	// ── Metric provider ───────────────────────────────────────────────────────
	// Always serve a Prometheus pull endpoint (/metrics); additionally push via
	// OTLP when a collector is configured so metrics reach the same pipeline as
	// traces and logs.
	promExp, err := promexporter.New()
	if err != nil {
		return nil, fmt.Errorf("creating Prometheus exporter: %w", err)
	}
	mpOpts := []sdkmetric.Option{
		sdkmetric.WithReader(promExp),
		sdkmetric.WithResource(res),
	}
	if conn != nil {
		metricExp, expErr := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
		if expErr != nil {
			return nil, fmt.Errorf("creating OTLP metric exporter: %w", expErr)
		}
		mpOpts = append(mpOpts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)))
	}
	mp := sdkmetric.NewMeterProvider(mpOpts...)
	otel.SetMeterProvider(mp)
	shutdownFns = append(shutdownFns, mp.Shutdown)

	// ── Log provider (OTLP) ───────────────────────────────────────────────────
	// Registered globally so NewLoggerWithExport can bridge slog records to it.
	// Structured JSON to stdout is retained regardless (see NewLoggerWithExport)
	// so `kubectl logs` / container stdout stay useful.
	if conn != nil {
		logExp, expErr := otlploggrpc.New(ctx, otlploggrpc.WithGRPCConn(conn))
		if expErr != nil {
			return nil, fmt.Errorf("creating OTLP log exporter: %w", expErr)
		}
		lp := sdklog.NewLoggerProvider(
			sdklog.WithResource(res),
			sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		)
		otellog.SetLoggerProvider(lp)
		shutdownFns = append(shutdownFns, lp.Shutdown)
	}

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

// parseLevel maps a level string to slog.Level, defaulting to Info.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewLogger returns a structured JSON slog.Logger that injects trace_id and
// span_id fields from the active span in the context on each log record. Used
// before Setup runs (and as the stdout half of NewLoggerWithExport).
func NewLogger(level string) *slog.Logger {
	h := &traceHandler{
		inner: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(level)}),
	}
	return slog.New(h)
}

// NewLoggerWithExport returns a logger that writes structured JSON to stdout
// (trace-correlated, so `kubectl logs` stays useful) and, when otlpEnabled,
// ALSO bridges every record to the global OTel LoggerProvider via otelslog so
// logs flow through the same OTLP pipeline as traces and metrics. Call after
// Setup so the LoggerProvider is registered.
func NewLoggerWithExport(level string, otlpEnabled bool) *slog.Logger {
	stdout := &traceHandler{
		inner: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(level)}),
	}
	if !otlpEnabled {
		return slog.New(stdout)
	}
	// The otelslog bridge extracts the active span context itself, so it does
	// not need the traceHandler wrapper.
	bridge := otelslog.NewHandler("ai-registry",
		otelslog.WithLoggerProvider(otellog.GetLoggerProvider()))
	return slog.New(NewFanoutHandler(stdout, bridge))
}

package store

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "ai-registry/store"

// startSpan starts a child span for a store (database) operation.
// The span is automatically attributed with db.system=postgresql.
// Callers must defer end() and call recordErr(err) before the deferred end.
//
// Typical usage:
//
//	ctx, end := startSpan(ctx, "ListMCPServers")
//	defer end()
//	... do work ...
//	if err != nil { recordErr(span, err) }
func startSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "db."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", operation),
		),
	)
	return ctx, span
}

// recordErr marks a span as errored if err is non-nil.
func recordErr(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

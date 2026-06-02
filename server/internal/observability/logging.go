package observability

import (
	"context"
	"errors"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// WrapWithTrace returns a slog.Handler that wraps inner and stamps
// trace_id / span_id attributes onto every record whose context carries a
// recording OTel span. Records emitted with no active span are passed
// through unchanged — no zero-valued trace_id/span_id pollution.
//
// Exported so tests can compose a buffer-backed JSON handler under the same
// trace-correlation behaviour that NewLogger applies to stdout.
func WrapWithTrace(inner slog.Handler) slog.Handler {
	return &traceHandler{inner: inner}
}

// traceHandler wraps a slog.Handler and adds trace_id / span_id attributes
// from the active OTel span in the context, enabling log-trace correlation.
type traceHandler struct {
	inner slog.Handler
}

func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		sc := span.SpanContext()
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{inner: h.inner.WithGroup(name)}
}

// fanoutHandler dispatches each record to every wrapped handler, so a single
// slog.Logger can write to stdout AND export via OTLP at once. Each handler
// receives its own record clone because handlers (e.g. traceHandler) mutate it.
type fanoutHandler struct {
	handlers []slog.Handler
}

// NewFanoutHandler returns a slog.Handler that dispatches every record to all
// of the given handlers. Exported so tests can exercise the fan-out contract
// without reaching into the package's private types.
func NewFanoutHandler(handlers ...slog.Handler) slog.Handler {
	return &fanoutHandler{handlers: handlers}
}

func (f *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (f *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &fanoutHandler{handlers: next}
}

func (f *fanoutHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithGroup(name)
	}
	return &fanoutHandler{handlers: next}
}

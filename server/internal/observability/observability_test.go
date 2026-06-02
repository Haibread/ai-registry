package observability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/haibread/ai-registry/internal/observability"
)

// The observability package was at 0.0% coverage in the post-Phase 7 audit
// despite carrying CLAUDE.md non-negotiables ("Structured logs must carry
// trace_id and span_id fields"; "Setup OTel providers"). These tests pin the
// public surface so a regression in the trace-log correlation, log-level
// parsing, or metric-instrument registration breaks CI rather than going
// undetected in production.

// ── NewLogger ───────────────────────────────────────────────────────────────

func TestNewLogger_LevelMapping(t *testing.T) {
	// We only assert that the returned logger reports the expected enabled
	// state for each level — the underlying writer (os.Stdout) is fixed so
	// we don't try to capture output here.
	cases := []struct {
		input string
		// debugEnabled is the easiest level to test against because it
		// changes for "debug" but is suppressed at "info"/"warn"/"error".
		debugEnabled bool
	}{
		{"debug", true},
		{"info", false},
		{"warn", false},
		{"error", false},
		{"unknown-level", false}, // default → info per the switch fallthrough
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			logger := observability.NewLogger(c.input)
			if got := logger.Enabled(context.Background(), slog.LevelDebug); got != c.debugEnabled {
				t.Errorf("level=%q: debug enabled = %v, want %v", c.input, got, c.debugEnabled)
			}
		})
	}
}

// ── traceHandler trace_id / span_id injection ──────────────────────────────

// withTraceHandler builds a slog.Logger backed by a buffer and the production
// traceHandler so we can inspect the JSON line emitted for a record. We can't
// reach into the unexported traceHandler directly, but NewLogger composes it
// with a JSON inner handler — same code path, same effective contract.
func withTraceHandler(t *testing.T, level slog.Level) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	// Mirror NewLogger's composition: traceHandler wraps a JSON handler.
	// We use a private buffer instead of os.Stdout because NewLogger writes
	// to stdout directly and assertions on stdout are racy across tests.
	// This relies on the public contract — the only way trace_id/span_id
	// appear is via the same traceHandler — but it doesn't reach across
	// the package boundary into private types.
	innerHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level})
	// Re-implement what NewLogger does, but parameterised so tests are
	// deterministic. If NewLogger ever changes signature, this test must be
	// updated; the behaviour we pin (trace_id+span_id injection from the
	// active OTel span) lives in the unexported traceHandler that the
	// public Setup wires up.
	logger := slog.New(observability.WrapWithTrace(innerHandler))
	return logger, &buf
}

func TestTraceHandler_InjectsTraceAndSpanIDs(t *testing.T) {
	// Install an in-memory tracer provider so SpanFromContext returns a
	// recording span — that's the precondition the traceHandler checks
	// before stamping trace_id / span_id onto the record.
	prevTP := otel.GetTracerProvider()
	t.Cleanup(func() { otel.SetTracerProvider(prevTP) })
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	otel.SetTracerProvider(tp)

	logger, buf := withTraceHandler(t, slog.LevelInfo)

	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	logger.InfoContext(ctx, "hello")
	span.End()

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("decode log line: %v\nraw=%s", err, buf.String())
	}
	if got, _ := rec["trace_id"].(string); got == "" || strings.HasPrefix(got, "00000000") {
		t.Errorf("trace_id missing or all-zero: %q", got)
	}
	if got, _ := rec["span_id"].(string); got == "" || strings.HasPrefix(got, "00000000") {
		t.Errorf("span_id missing or all-zero: %q", got)
	}
}

func TestTraceHandler_NoSpanContextNoIDs(t *testing.T) {
	// With no recording span on the context, the handler must NOT stamp
	// zero-valued trace_id / span_id onto the record. Polluting log lines
	// with "00000000…" entries breaks log-correlation queries.
	logger, buf := withTraceHandler(t, slog.LevelInfo)

	logger.Info("no-span") // top-level — no context

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("decode log line: %v\nraw=%s", err, buf.String())
	}
	if _, ok := rec["trace_id"]; ok {
		t.Errorf("trace_id was set when no span exists: %v", rec["trace_id"])
	}
	if _, ok := rec["span_id"]; ok {
		t.Errorf("span_id was set when no span exists: %v", rec["span_id"])
	}
}

func TestTraceHandler_PreservesUserAttrs(t *testing.T) {
	// The handler must add the trace IDs WITHOUT replacing or shadowing
	// caller-supplied attributes. A drop-on-the-floor regression here would
	// be invisible in normal logs and expensive to debug after the fact.
	logger, buf := withTraceHandler(t, slog.LevelInfo)

	logger.Info("with-attrs", slog.String("user_id", "u-42"), slog.Int("count", 7))

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("decode log line: %v\nraw=%s", err, buf.String())
	}
	if rec["user_id"] != "u-42" {
		t.Errorf("user_id = %v, want u-42", rec["user_id"])
	}
	// JSON numeric is float64 by default in Go's encoding/json.
	if got, _ := rec["count"].(float64); got != 7 {
		t.Errorf("count = %v, want 7", rec["count"])
	}
}

// ── NewLoggerWithExport / fanout ────────────────────────────────────────────

func TestNewLoggerWithExport_NoOTLPStillLogsToStdout(t *testing.T) {
	// With otlpEnabled=false the function must return a working stdout logger
	// (the no-collector deployment must not lose logs). We only assert the
	// level contract here since the writer is os.Stdout.
	logger := observability.NewLoggerWithExport("debug", false)
	if logger == nil {
		t.Fatal("NewLoggerWithExport returned nil")
	}
	if !logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug level should be enabled for level=debug")
	}
}

func TestFanoutHandler_DispatchesToEveryHandler(t *testing.T) {
	// The fanout handler underpins "stdout JSON AND OTLP": a single record must
	// reach every wrapped handler, each getting an independent clone so one
	// handler mutating the record (traceHandler stamps trace_id) can't corrupt
	// another's copy.
	var bufA, bufB bytes.Buffer
	a := slog.NewJSONHandler(&bufA, &slog.HandlerOptions{Level: slog.LevelInfo})
	// Wrap one side in the production traceHandler, which mutates the record.
	b := observability.WrapWithTrace(slog.NewJSONHandler(&bufB, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger := slog.New(observability.NewFanoutHandler(a, b))

	logger.Info("fanned-out", slog.String("k", "v"))

	for name, buf := range map[string]*bytes.Buffer{"A": &bufA, "B": &bufB} {
		var rec map[string]any
		if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
			t.Fatalf("handler %s: decode: %v\nraw=%s", name, err, buf.String())
		}
		if rec["msg"] != "fanned-out" || rec["k"] != "v" {
			t.Errorf("handler %s: got %v, want msg=fanned-out k=v", name, rec)
		}
	}
}

// ── InitMetrics ────────────────────────────────────────────────────────────

func TestInitMetrics_ReturnsAllInstruments(t *testing.T) {
	// InitMetrics resolves the meter from the global MeterProvider and
	// registers every instrument the application uses. We don't verify the
	// names against Prometheus output here — that needs a Prometheus
	// exporter and is exercised via /metrics in the integration tests.
	// The contract this test pins: every field on *Metrics is non-nil,
	// because handlers dereference them without nil-checking.
	m, err := observability.InitMetrics()
	if err != nil {
		t.Fatalf("InitMetrics: %v", err)
	}
	if m == nil {
		t.Fatal("InitMetrics returned a nil *Metrics with no error")
	}
	// Each instrument must be non-nil; a nil counter would panic on first
	// use in the handlers.
	if m.HTTPRequestsTotal == nil {
		t.Error("HTTPRequestsTotal is nil")
	}
	if m.HTTPRequestDuration == nil {
		t.Error("HTTPRequestDuration is nil")
	}
	if m.MCPServersTotal == nil {
		t.Error("MCPServersTotal is nil")
	}
	if m.AgentsTotal == nil {
		t.Error("AgentsTotal is nil")
	}
	if m.AuthFailures == nil {
		t.Error("AuthFailures is nil")
	}
	if m.RateLimitHits == nil {
		t.Error("RateLimitHits is nil")
	}
}

func TestInitMetrics_Idempotent(t *testing.T) {
	// Two consecutive calls must both succeed — the OTel SDK reuses
	// instrument registrations under the same name. Tests that swap the
	// global provider multiple times rely on this.
	if _, err := observability.InitMetrics(); err != nil {
		t.Fatalf("InitMetrics call 1: %v", err)
	}
	if _, err := observability.InitMetrics(); err != nil {
		t.Errorf("InitMetrics call 2: %v", err)
	}
}

package headers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	rrcontext "github.com/roadrunner-server/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	jprop "go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestMiddleware_SpanEndBeforeNext(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	t.Cleanup(func() {
		_ = tp.Shutdown(t.Context())
	})

	p := newTestPlugin()

	// "next" handler creates its own span so we can assert ordering.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, nextSpan := tp.Tracer("test-tracer").Start(r.Context(), "next-handler")
		defer nextSpan.End()
		w.WriteHeader(http.StatusOK)
	})

	ctx := context.WithValue(t.Context(), rrcontext.OtelTracerNameKey, "test-tracer")
	parentCtx, parentSpan := tp.Tracer("test-tracer").Start(ctx, "parent")
	defer parentSpan.End()

	req := httptest.NewRequestWithContext(parentCtx, http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	p.Middleware(next).ServeHTTP(rec, req)

	require.NoError(t, tp.ForceFlush(t.Context()))

	spans := exporter.GetSpans()

	headersSpan := findSpan(spans, PluginName)
	nextSpan := findSpan(spans, "next-handler")

	require.NotNil(t, headersSpan, "expected a span named %q", PluginName)
	require.NotNil(t, nextSpan, "expected a span named %q", "next-handler")

	// The headers span must end before (or at the same time as) the next handler starts,
	// proving the span covers only the middleware's own work.
	assert.False(t, headersSpan.EndTime.After(nextSpan.StartTime),
		"headers span EndTime (%s) should not be after next-handler StartTime (%s)",
		headersSpan.EndTime, nextSpan.StartTime)

	assert.Equal(t, trace.SpanKindInternal, headersSpan.SpanKind)
}

func TestMiddleware_NoSpanWithoutOtelContext(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	t.Cleanup(func() {
		_ = tp.Shutdown(t.Context())
	})

	p := newTestPlugin()

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// No OtelTracerNameKey in context — no span should be created.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	p.Middleware(next).ServeHTTP(rec, req)

	require.NoError(t, tp.ForceFlush(t.Context()))

	spans := exporter.GetSpans()
	headersSpan := findSpan(spans, PluginName)
	assert.Nil(t, headersSpan, "no span should be created without OtelTracerNameKey")
}

func newTestPlugin() *Plugin {
	return &Plugin{
		cfg: &Config{
			Request:  map[string]string{"X-Req": "value"},
			Response: map[string]string{"X-Resp": "value"},
		},
		prop: propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{}, jprop.Jaeger{},
		),
	}
}

func findSpan(spans []tracetest.SpanStub, name string) *tracetest.SpanStub {
	for i := range spans {
		if spans[i].Name == name {
			return &spans[i]
		}
	}
	return nil
}

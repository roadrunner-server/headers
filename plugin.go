package headers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/roadrunner-server/api/v2/plugins/config"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v2/utils"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	jprop "go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
)

// PluginName contains default service name.
const (
	RootPluginName string = "http"
	PluginName     string = "headers"
)

// Plugin serves headers files. Potentially convert into middleware?
type Plugin struct {
	// server configuration (location, forbidden files and etc)
	cfg *Config

	prop propagation.TextMapPropagator
}

// Init must return configure service and return true if service hasStatus enabled. Must return error in case of
// misconfiguration. Services must not be used without proper configuration pushed first.
func (p *Plugin) Init(cfg config.Configurer) error {
	const op = errors.Op("headers_plugin_init")

	if !cfg.Has(RootPluginName) {
		return errors.E(op, errors.Disabled)
	}

	if !cfg.Has(fmt.Sprintf("%s.%s", RootPluginName, PluginName)) {
		return errors.E(op, errors.Disabled)
	}

	err := cfg.UnmarshalKey(fmt.Sprintf("%s.%s", RootPluginName, PluginName), &p.cfg)
	if err != nil {
		return errors.E(op, err)
	}

	p.prop = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}, jprop.Jaeger{})

	return nil
}

// Middleware is HTTP plugin middleware to serve headers
func (p *Plugin) Middleware(next http.Handler) http.Handler {
	// Define the http.HandlerFunc
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if val, ok := r.Context().Value(utils.OtelTracerNameKey).(string); ok {
			tp := trace.SpanFromContext(r.Context()).TracerProvider()
			ctx, span := tp.Tracer(val, trace.WithSchemaURL(semconv.SchemaURL),
				trace.WithInstrumentationVersion(otelhttp.SemVersion())).
				Start(r.Context(), PluginName, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()

			// inject
			p.prop.Inject(ctx, propagation.HeaderCarrier(r.Header))
			r = r.WithContext(ctx)
		}

		if p.cfg.Request != nil {
			for k, v := range p.cfg.Request {
				r.Header.Add(k, v)
			}
		}

		if p.cfg.Response != nil {
			for k, v := range p.cfg.Response {
				w.Header().Set(k, v)
			}
		}

		if p.cfg.CORS != nil {
			if r.Method == http.MethodOptions {
				p.preflightRequest(w)

				return
			}
			p.corsHeaders(w)
		}

		next.ServeHTTP(w, r)
	})
}

func (p *Plugin) Name() string {
	return PluginName
}

// configure OPTIONS response
func (p *Plugin) preflightRequest(w http.ResponseWriter) {
	headers := w.Header()

	headers.Add("Vary", "Origin")
	headers.Add("Vary", "Access-Control-Request-Method")
	headers.Add("Vary", "Access-Control-Request-Headers")

	if p.cfg.CORS.AllowedOrigin != "" {
		headers.Set("Access-Control-Allow-Origin", p.cfg.CORS.AllowedOrigin)
	}

	if p.cfg.CORS.AllowedHeaders != "" {
		headers.Set("Access-Control-Allow-Headers", p.cfg.CORS.AllowedHeaders)
	}

	if p.cfg.CORS.AllowedMethods != "" {
		headers.Set("Access-Control-Allow-Methods", p.cfg.CORS.AllowedMethods)
	}

	if p.cfg.CORS.AllowCredentials != nil {
		headers.Set("Access-Control-Allow-Credentials", strconv.FormatBool(*p.cfg.CORS.AllowCredentials))
	}

	if p.cfg.CORS.MaxAge > 0 {
		headers.Set("Access-Control-Max-Age", strconv.Itoa(p.cfg.CORS.MaxAge))
	}

	w.WriteHeader(http.StatusOK)
}

// configure CORS headers
func (p *Plugin) corsHeaders(w http.ResponseWriter) {
	headers := w.Header()

	headers.Add("Vary", "Origin")

	if p.cfg.CORS.AllowedOrigin != "" {
		headers.Set("Access-Control-Allow-Origin", p.cfg.CORS.AllowedOrigin)
	}

	if p.cfg.CORS.AllowedHeaders != "" {
		headers.Set("Access-Control-Allow-Headers", p.cfg.CORS.AllowedHeaders)
	}

	if p.cfg.CORS.ExposedHeaders != "" {
		headers.Set("Access-Control-Expose-Headers", p.cfg.CORS.ExposedHeaders)
	}

	if p.cfg.CORS.AllowCredentials != nil {
		headers.Set("Access-Control-Allow-Credentials", strconv.FormatBool(*p.cfg.CORS.AllowCredentials))
	}
}

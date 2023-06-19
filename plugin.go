package headers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v4/utils"
	"github.com/rs/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	jprop "go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
)

// PluginName contains default service name.
const (
	RootPluginName string = "http"
	PluginName     string = "headers"
)

type Configurer interface {
	// UnmarshalKey takes a single key and unmarshal it into a Struct.
	UnmarshalKey(name string, out any) error
	// Has checks if config section exists.
	Has(name string) bool
}

// Plugin serves headers files. Potentially convert into middleware?
type Plugin struct {
	// server configuration (location, forbidden files and etc)
	cfg  *Config
	prop propagation.TextMapPropagator
	cors *cors.Cors
}

// Init must return configure service and return true if service hasStatus enabled. Must return error in case of
// misconfiguration. Services must not be used without proper configuration pushed first.
func (p *Plugin) Init(cfg Configurer) error {
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

	// Configure CORS options
	if p.cfg.CORS != nil {
		opts := cors.Options{
			// Keep BC with previous implementation
			OptionsSuccessStatus: http.StatusOK,
			Debug:                p.cfg.CORS.Debug,
		}

		if p.cfg.CORS.AllowedOrigin != "" {
			opts.AllowedOrigins = strings.Split(p.cfg.CORS.AllowedOrigin, ",")
		}

		if p.cfg.CORS.AllowedMethods != "" {
			opts.AllowedMethods = strings.Split(p.cfg.CORS.AllowedMethods, ",")
		}

		if p.cfg.CORS.AllowedHeaders != "" {
			opts.AllowedHeaders = strings.Split(p.cfg.CORS.AllowedHeaders, ",")
		}

		if p.cfg.CORS.ExposedHeaders != "" {
			opts.ExposedHeaders = strings.Split(p.cfg.CORS.ExposedHeaders, ",")
		}

		if p.cfg.CORS.MaxAge > 0 {
			opts.MaxAge = p.cfg.CORS.MaxAge
		}

		opts.AllowCredentials = p.cfg.CORS.AllowCredentials

		if p.cfg.CORS.OptionsSuccessStatus != 0 {
			opts.OptionsSuccessStatus = p.cfg.CORS.OptionsSuccessStatus
		}

		p.cors = cors.New(opts)
	}

	return nil
}

// Middleware is HTTP plugin middleware to serve headers
func (p *Plugin) Middleware(next http.Handler) http.Handler {
	// Configure CORS handler
	if p.cors != nil {
		next = p.cors.Handler(next)
	}

	// Define the http.HandlerFunc
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if val, ok := r.Context().Value(utils.OtelTracerNameKey).(string); ok {
			tp := trace.SpanFromContext(r.Context()).TracerProvider()
			ctx, span := tp.Tracer(val, trace.WithSchemaURL(semconv.SchemaURL),
				trace.WithInstrumentationVersion(otelhttp.Version())).
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

		next.ServeHTTP(w, r)
	})
}

func (p *Plugin) Name() string {
	return PluginName
}

module github.com/roadrunner-server/headers/v3

go 1.19

require (
	github.com/roadrunner-server/errors v1.2.0
	github.com/roadrunner-server/sdk/v3 v3.0.0-beta.5
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.36.4
	go.opentelemetry.io/contrib/propagators/jaeger v1.11.1
	go.opentelemetry.io/otel v1.11.1
	go.opentelemetry.io/otel/trace v1.11.1
)

require (
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/roadrunner-server/tcplisten v1.2.0 // indirect
	go.opentelemetry.io/otel/metric v0.33.0 // indirect
)

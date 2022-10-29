module github.com/zakharovvi/aws-lambda-extensions/examples/logs-subscriber/extension

go 1.18

// can't use relative path as "sam build" copies sources to temp directory before calling "go build"
replace github.com/zakharovvi/aws-lambda-extensions => /Users/zakharovvi/go/src/github.com/zakharovvi/aws-lambda-extensions

require (
	github.com/go-logr/stdr v1.2.2
	github.com/zakharovvi/aws-lambda-extensions v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/otel v1.11.1
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v0.33.0
	go.opentelemetry.io/otel/metric v0.33.0
	go.opentelemetry.io/otel/sdk v1.11.1
	go.opentelemetry.io/otel/sdk/metric v0.33.0
)

require (
	github.com/go-logr/logr v1.2.3 // indirect
	go.opentelemetry.io/otel/trace v1.11.1 // indirect
	golang.org/x/sys v0.0.0-20220919091848-fb04ddd9f9c8 // indirect
)

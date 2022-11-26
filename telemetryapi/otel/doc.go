// Package otel implements conversion from Telemetry API events into OpenTelemetry trace spans.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-otel-spans.html
//
// Package otel can be used with OpenTelemetry compatible exporter to send traces to any destinations.
// https://github.com/open-telemetry/opentelemetry-go/tree/main/exporters
//
// End-to-end example is available in examples/telemetry-otel-trace-exporter
package otel

// Sample extension to demonstrate how to convert Telemetry API events into OpenTelemetry tracing spans.
package main

import (
	"context"
	"log"
	"os"

	"github.com/go-logr/stdr"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
)

func main() {
	ctx := context.Background()

	// log library debug messages
	stdr.SetVerbosity(1)
	logger := stdr.New(log.New(os.Stdout, "", log.Lshortfile))

	exporter, err := stdouttrace.New()
	if err != nil {
		log.Panic(err)
	}
	processor := otel.NewProcessor(ctx, exporter, otel.WithLogger(logger))

	if err := telemetryapi.Run(
		ctx,
		processor,
		telemetryapi.WithSubscriptionTypes([]extapi.TelemetrySubscriptionType{extapi.TelemetrySubscriptionTypePlatform}),
		telemetryapi.WithLogger(logger),
		telemetryapi.WithBufferingCfg(&extapi.TelemetryBufferingCfg{TimeoutMS: 25, MaxBytes: 262144, MaxItems: 1000}),
	); err != nil {
		log.Panic(err)
	}
}

package otel_test

import (
	"context"
	"log"

	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
)

func ExampleProcessor() {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Panic(err)
	}

	ctx := context.Background()
	processor := otel.NewProcessor(ctx, exporter)

	if err := telemetryapi.Run(ctx, processor); err != nil {
		log.Panic(err)
	}
}

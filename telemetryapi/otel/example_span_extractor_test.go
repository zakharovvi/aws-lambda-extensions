package otel_test

import (
	"context"
	"log"

	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
)

func ExampleSpanConverter() {
	ctx := context.Background()
	// 1. get metadata
	// In real-world it is done in telemetryapi.Processor.Process()
	registerResp := &extapi.RegisterResponse{}

	// 2. create span spanConverter
	spanConverter := otel.NewSpanConverter(ctx, registerResp)

	// 3. receive events.
	// In real-world it is done in telemetryapi.Processor.Process()
	triplet := getInvokeTriplet()

	// 4. convert events into opentelemetry spans
	spans, _, err := spanConverter.ConvertIntoSpans(triplet)
	if err != nil {
		log.Panic(err)
	}

	// 5. send events to sdktrace.SpanExporter
	// https://github.com/open-telemetry/opentelemetry-go/tree/main/exporters
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Panic(err)
	}
	if err := exporter.ExportSpans(ctx, spans); err != nil {
		log.Panic(err)
	}
	if err := exporter.Shutdown(ctx); err != nil {
		log.Panic(err)
	}
}

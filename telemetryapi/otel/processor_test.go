package otel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestProcessor_Process_Link(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	exporter := tracetest.NewInMemoryExporter()
	proc := otel.NewProcessor(ctx, exporter)

	err := proc.Init(ctx, registerResp)
	require.NoError(t, err)

	initTriplet := getInitTriplet()
	err = proc.Process(ctx, initTriplet.Start)
	require.NoError(t, err)
	err = proc.Process(ctx, initTriplet.RuntimeDone)
	require.NoError(t, err)
	err = proc.Process(ctx, initTriplet.Report)
	require.NoError(t, err)

	invokeTriplet := getInvokeTriplet()
	err = proc.Process(ctx, invokeTriplet.Start)
	require.NoError(t, err)
	err = proc.Process(ctx, invokeTriplet.RuntimeDone)
	require.NoError(t, err)
	err = proc.Process(ctx, invokeTriplet.Report)
	require.NoError(t, err)

	require.Len(t, exporter.GetSpans(), 4)
	var initSpanContext trace.SpanContext
	var invokeLinkSpanContext trace.SpanContext
	for _, span := range exporter.GetSpans() {
		if span.Name == "test-name/init" {
			initSpanContext = span.SpanContext
		}
		if span.Name == "test-name/invoke" {
			invokeLinkSpanContext = span.Links[0].SpanContext
		}
	}
	require.NotEmpty(t, initSpanContext)
	require.Equal(t, initSpanContext, invokeLinkSpanContext)

	err = proc.Shutdown(ctx, extapi.Spindown, nil)
	require.NoError(t, err)
}

func TestProcessor_Process_OutOfOrder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	exporter := tracetest.NewInMemoryExporter()
	proc := otel.NewProcessor(ctx, exporter)

	err := proc.Init(ctx, registerResp)
	require.NoError(t, err)

	initTriplet := getInitTriplet()
	err = proc.Process(ctx, initTriplet.Report)
	require.Error(t, err)
}

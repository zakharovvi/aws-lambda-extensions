package internal

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// IDGenerator returns predefined traceID and spanID when they exist instead of generating new random IDs every time.
// IDGenerator is used to convert Telemetry API events into trace spans using
// OpenTelemetry SDK, which provides very limited access to create and manipulate span data.
type IDGenerator struct {
	nextTraceID trace.TraceID
	nextSpanID  trace.SpanID
	Gen         sdktrace.IDGenerator
}

func (g *IDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	if g.nextTraceID.IsValid() && g.nextSpanID.IsValid() {
		traceID, spanID := g.nextTraceID, g.nextSpanID
		g.nextTraceID = trace.TraceID{}
		g.nextSpanID = trace.SpanID{}

		return traceID, spanID
	}

	return g.Gen.NewIDs(ctx)
}

func (g *IDGenerator) NewSpanID(ctx context.Context, traceID trace.TraceID) trace.SpanID {
	if g.nextTraceID.IsValid() && g.nextSpanID.IsValid() {
		spanID := g.nextSpanID
		g.nextTraceID = trace.TraceID{}
		g.nextSpanID = trace.SpanID{}

		return spanID
	}

	return g.Gen.NewSpanID(ctx, traceID)
}

func (g *IDGenerator) SetNext(nextTraceID trace.TraceID, nextSpanID trace.SpanID) {
	g.nextTraceID = nextTraceID
	g.nextSpanID = nextSpanID
}

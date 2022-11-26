package otel

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Processor implements telemetryapi.Processor interface to export Telemetry API events as OpenTelemetry spans
// through a given exporter.
// Processor should be passed into telemetryapi.Run instead of direct usage.
type Processor struct {
	exporter      sdktrace.SpanExporter
	log           logr.Logger
	spanConverter *SpanConverter
	opts          []Option
	curTriplet    EventTriplet
}

// NewProcessor creates Processor with provided sdktrace.SpanExporter.
func NewProcessor(ctx context.Context, exporter sdktrace.SpanExporter, opts ...Option) *Processor {
	options := options{
		log: logr.FromContextOrDiscard(ctx),
	}
	for _, o := range opts {
		o.apply(&options)
	}

	return &Processor{exporter: exporter, log: options.log, opts: opts}
}

func (proc *Processor) Init(ctx context.Context, registerResp *extapi.RegisterResponse) error {
	proc.spanConverter = NewSpanConverter(ctx, registerResp, proc.opts...)

	return nil
}

func (proc *Processor) Process(ctx context.Context, event telemetryapi.Event) error {
	switch event.Record.(type) {
	case telemetryapi.RecordPlatformInitStart:
		proc.curTriplet.Type = telemetryapi.PhaseInit
		proc.curTriplet.Start = event
	case telemetryapi.RecordPlatformInitRuntimeDone:
		proc.curTriplet.RuntimeDone = event
	case telemetryapi.RecordPlatformInitReport:
		proc.curTriplet.Report = event
		spanContext, err := proc.exportTriplet(ctx)
		if err != nil {
			return err
		}
		proc.curTriplet = EventTriplet{PrevSC: spanContext}
	case telemetryapi.RecordPlatformStart:
		proc.curTriplet.Type = telemetryapi.PhaseInvoke
		proc.curTriplet.Start = event
	case telemetryapi.RecordPlatformRuntimeDone:
		proc.curTriplet.RuntimeDone = event
	case telemetryapi.RecordPlatformReport:
		proc.curTriplet.Report = event
		spanContext, err := proc.exportTriplet(ctx)
		if err != nil {
			return err
		}
		proc.curTriplet = EventTriplet{PrevSC: spanContext}
	}

	return nil
}

func (proc *Processor) exportTriplet(ctx context.Context) (trace.SpanContext, error) {
	spans, spanContext, err := proc.spanConverter.ConvertIntoSpans(proc.curTriplet)
	if err != nil {
		return spanContext, err
	}

	proc.log.V(1).Info(
		"sending spans to exporter",
		"traceID", spanContext.TraceID(),
		"count", len(spans),
	)

	return spanContext, proc.exporter.ExportSpans(ctx, spans)
}

func (proc *Processor) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	proc.log.V(1).Info("shutting down span exporter")

	return proc.exporter.Shutdown(ctx)
}

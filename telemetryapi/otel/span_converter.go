package otel

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	lambdaext "github.com/zakharovvi/aws-lambda-extensions"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi/otel/internal"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
)

// SpanConverter creates OpenTelemetry spans from Telemetry API events.
// SpanConverter is low-level, consider using Processor instead.
type SpanConverter struct {
	tracer       trace.Tracer
	gen          *internal.IDGenerator
	log          logr.Logger
	functionName string
}

type Option interface {
	apply(*options)
}

type options struct {
	log logr.Logger
}

type loggerOption struct {
	log logr.Logger
}

func (o loggerOption) apply(opts *options) {
	opts.log = o.log
}

func WithLogger(log logr.Logger) Option {
	return loggerOption{log}
}

// NewSpanConverter creates SpanConverter.
func NewSpanConverter(ctx context.Context, registerResp *extapi.RegisterResponse, opts ...Option) *SpanConverter {
	options := options{
		log: logr.FromContextOrDiscard(ctx),
	}
	for _, o := range opts {
		o.apply(&options)
	}

	otel.SetLogger(options.log)
	gen := &internal.IDGenerator{
		Gen: xray.NewIDGenerator(),
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithIDGenerator(gen),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.CloudProviderAWS,
			semconv.CloudPlatformAWSLambda,
			semconv.CloudAccountIDKey.String(registerResp.AccountID),
			semconv.CloudRegionKey.String(extapi.EnvAWSRegion()),
			semconv.FaaSNameKey.String(registerResp.FunctionName),
			semconv.FaaSVersionKey.String(string(registerResp.FunctionVersion)),
			semconv.FaaSMaxMemoryKey.Int(extapi.EnvAWSLambdaFunctionMemorySizeMB()),
		)),
	)
	tracer := tp.Tracer("github.com/zakharovvi/aws-lambda-extensions/telemetryapi/otel")

	return &SpanConverter{
		tracer,
		gen,
		options.log,
		registerResp.FunctionName,
	}
}

// EventTriplet contains chain of events from single Lambda function invocation.
type EventTriplet struct {
	Type        telemetryapi.Phase
	Start       telemetryapi.Event
	RuntimeDone telemetryapi.Event
	Report      telemetryapi.Event
	PrevSC      trace.SpanContext
}

// IsValid checks that received events match and in-order.
func (t EventTriplet) IsValid() bool {
	switch t.Type {
	case telemetryapi.PhaseInit:
		if t.Start.Type != telemetryapi.TypePlatformInitStart {
			return false
		}
		if t.RuntimeDone.Type != telemetryapi.TypePlatformInitRuntimeDone {
			return false
		}
		if t.Report.Type != telemetryapi.TypePlatformInitReport {
			return false
		}
	case telemetryapi.PhaseInvoke:
		if t.Start.Type != telemetryapi.TypePlatformStart {
			return false
		}
		if t.RuntimeDone.Type != telemetryapi.TypePlatformRuntimeDone {
			return false
		}
		if t.Report.Type != telemetryapi.TypePlatformReport {
			return false
		}
	default:
		return false
	}

	return true
}

// ConvertIntoSpans creates OpenTelemetry spans from provided triplet of Telemetry API events.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-otel-spans.html
func (sc *SpanConverter) ConvertIntoSpans(triplet EventTriplet) ([]sdktrace.ReadOnlySpan, trace.SpanContext, error) {
	if !triplet.IsValid() {
		return nil, trace.SpanContext{}, fmt.Errorf("received triplet is not consistent: events were received out of order")
	}

	parentCtx := context.Background()
	if record, ok := triplet.Start.Record.(telemetryapi.RecordPlatformStart); ok {
		carrier := propagation.MapCarrier{
			string(record.Tracing.Type): string(record.Tracing.Value),
		}
		parentCtx = xray.Propagator{}.Extract(context.Background(), carrier)
		spanID, err := trace.SpanIDFromHex(record.Tracing.SpanID)
		if err == nil {
			traceID := trace.SpanContextFromContext(parentCtx).TraceID()
			sc.log.V(1).Info("found xray tracing context", "traceID", traceID, "parentSpanID", spanID)
			sc.gen.SetNext(traceID, spanID)
		} else {
			sc.log.V(1).Info("xray tracing is not enabled")
		}
	}

	var links []trace.Link
	if triplet.PrevSC.HasSpanID() {
		sc.log.V(1).Info("link previous trace", "prevTraceID", triplet.PrevSC.TraceID(), "prevSpanID", triplet.PrevSC.SpanID())
		link := trace.Link{
			SpanContext: triplet.PrevSC,
			Attributes:  []attribute.KeyValue{attribute.String("aws.lambda.link_type", "previous-trace")},
		}
		links = append(links, link)
	}

	spanName := fmt.Sprintf("%s/%s", sc.functionName, triplet.Type)
	curCtx, span := sc.tracer.Start(
		parentCtx,
		spanName,
		trace.WithTimestamp(triplet.Start.Time),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(getAttributes(triplet)...),
		trace.WithLinks(links...),
	)
	sc.log.V(1).Info(
		"created span",
		"name", spanName,
		"traceID", span.SpanContext().TraceID(),
		"spanID", span.SpanContext().SpanID(),
	)

	status, err := getStatus(triplet.RuntimeDone)
	if err != nil {
		return nil, trace.SpanContext{}, err
	}
	span.SetStatus(status.Code, status.Description)

	var spans []sdktrace.ReadOnlySpan
	if record, ok := triplet.RuntimeDone.Record.(telemetryapi.RecordPlatformRuntimeDone); ok {
		var err error
		spans, err = sc.createChildSpans(curCtx, record)
		if err != nil {
			return nil, trace.SpanContext{}, err
		}
	}

	span.End(trace.WithTimestamp(triplet.Report.Time))
	roSpan, ok := span.(sdktrace.ReadOnlySpan)
	if !ok {
		return nil, trace.SpanContext{}, fmt.Errorf("could not cast span to ReadOnlySpan")
	}
	spans = append(spans, roSpan)

	return spans, trace.SpanContextFromContext(curCtx), nil
}

func (sc *SpanConverter) createChildSpans(ctx context.Context, record telemetryapi.RecordPlatformRuntimeDone) ([]sdktrace.ReadOnlySpan, error) {
	spans := make([]sdktrace.ReadOnlySpan, 0, len(record.Spans))
	for _, recordSpan := range record.Spans {
		spanName := fmt.Sprintf("%s/%s", sc.functionName, recordSpan.Name)
		_, childSpan := sc.tracer.Start(
			ctx,
			spanName,
			trace.WithTimestamp(recordSpan.Start),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		childSpan.End(trace.WithTimestamp(recordSpan.Start.Add(time.Duration(recordSpan.Duration))))
		sc.log.V(1).Info(
			"created child span",
			"name", spanName,
			"traceID", childSpan.SpanContext().TraceID(),
			"spanID", childSpan.SpanContext().SpanID(),
		)

		span, ok := childSpan.(sdktrace.ReadOnlySpan)
		if !ok {
			return nil, fmt.Errorf("could not cast childSpan to ReadOnlySpan")
		}
		spans = append(spans, span)
	}

	return spans, nil
}

func getAttributes(triplet EventTriplet) []attribute.KeyValue {
	var attrs []attribute.KeyValue

	if record, ok := triplet.Start.Record.(telemetryapi.RecordPlatformInitStart); ok {
		var coldStart bool
		if record.InitType == lambdaext.InitTypeOnDemand {
			coldStart = true
		}
		attrs = append(attrs, semconv.FaaSColdstartKey.Bool(coldStart))

		if record.RuntimeVersion != "" {
			attrs = append(attrs, attribute.String("aws.lambda.runtime_version", record.RuntimeVersion))
		}

		if record.RuntimeVersionARN != "" {
			attrs = append(attrs, attribute.String("aws.lambda.runtime_version_arn", record.RuntimeVersionARN))
		}
	}

	if record, ok := triplet.Start.Record.(telemetryapi.RecordPlatformStart); ok {
		attrs = append(attrs, semconv.FaaSExecutionKey.String(string(record.RequestID)))
	}

	if record, ok := triplet.RuntimeDone.Record.(telemetryapi.RecordPlatformRuntimeDone); ok {
		attrs = append(attrs, attribute.Int("aws.lambda.produced_bytes", record.Metrics.ProducedBytes))
	}

	if record, ok := triplet.Report.Record.(telemetryapi.RecordPlatformReport); ok {
		attrs = append(
			attrs,
			attribute.Int("aws.lambda.memory_size_mb", record.Metrics.MemorySizeMB),
			attribute.Int("aws.lambda.max_memory_used_mb", record.Metrics.MaxMemoryUsedMB),
			attribute.Int64("aws.lambda.billed_duration_ms", time.Duration(record.Metrics.BilledDuration).Milliseconds()),
		)
		if record.Metrics.RestoreDuration != 0 {
			attribute.Int64("aws.lambda.restore_duration_ms", time.Duration(record.Metrics.RestoreDuration).Milliseconds())
		}
	}

	return attrs
}

func getStatus(event telemetryapi.Event) (sdktrace.Status, error) {
	var eventStatus telemetryapi.Status
	status := sdktrace.Status{}

	switch record := event.Record.(type) {
	case telemetryapi.RecordPlatformInitRuntimeDone:
		eventStatus = record.Status
		status.Description = record.ErrorType
	case telemetryapi.RecordPlatformRuntimeDone:
		eventStatus = record.Status
		status.Description = record.ErrorType
	default:
		return status, fmt.Errorf("unexpected type for triplet.RuntimeDone field")
	}

	if eventStatus == telemetryapi.StatusSuccess {
		status.Code = codes.Ok
	} else {
		status.Code = codes.Error
	}

	return status, nil
}

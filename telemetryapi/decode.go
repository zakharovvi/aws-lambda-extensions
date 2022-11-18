package telemetryapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	lambdaext "github.com/zakharovvi/aws-lambda-extensions"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/internal"
)

// Type details the types of Event objects that the Lambda Telemetry API supports.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html
type Type string

const (
	// TypePlatformInitStart event is emitted when function initialization started.
	TypePlatformInitStart Type = "platform.initStart"
	// TypePlatformInitRuntimeDone event is emitted when function initialization completed.
	TypePlatformInitRuntimeDone Type = "platform.initRuntimeDone"
	// TypePlatformInitReport event is a report of function initialization.
	TypePlatformInitReport Type = "platform.initReport"
	// TypePlatformStart event is emitted when function invocation started.
	TypePlatformStart Type = "platform.start"
	// TypePlatformRuntimeDone event is emitted when the runtime finished processing an event with either success or failure.
	TypePlatformRuntimeDone Type = "platform.runtimeDone"
	// TypePlatformReport event is a report of function invocation.
	TypePlatformReport Type = "platform.report"
	// TypePlatformExtension event is emitted when an extension registers with the extensions API.
	TypePlatformExtension = "platform.extension"
	// TypePlatformTelemetrySubscription event is emitted when an extension subscribed to the Telemetry API.
	TypePlatformTelemetrySubscription Type = "platform.telemetrySubscription"
	// TypePlatformLogsDropped event is mmited when lambda dropped log entries.
	TypePlatformLogsDropped Type = "platform.logsDropped"
	// TypeFunction event is a log line from function code.
	TypeFunction Type = "function"
	// TypeExtension event is a log line from extension code.
	TypeExtension Type = "extension"
)

// Event object that the Lambda Telemetry API supports.
// After subscribing using the Telemetry API, an extension automatically starts to receive telemetry from Lambda.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-api.html#telemetry-api-messages
type Event struct {
	// Type property defines the event type.
	Type Type `json:"type"`
	// Time property defines when the Lambda platform generated the event.
	// This isn't the same as when the event actually occurred.
	// The string value of time is a timestamp in ISO 8601 format.
	Time time.Time `json:"time"`
	// RawRecord property defines a JSON object that contains the telemetry data.
	// The schema of this JSON object depends on the type.
	RawRecord json.RawMessage `json:"record"`
	// Record property defines a struct that contains the telemetry data.
	// The type of the struct depends on the Event.Type
	Record any `json:"decodedRecord,omitempty"` // tag for printing the field with json.Marshal
}

// RecordPlatformInitStart event indicates that the function initialization phase has started.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#platform-initStart
type RecordPlatformInitStart struct {
	InitType          lambdaext.InitType `json:"initializationType"`
	Phase             Phase              `json:"phase"`
	RuntimeVersion    string             `json:"runtimeVersion,omitempty"`
	RuntimeVersionARN string             `json:"runtimeVersionArn,omitempty"`
}

// RecordPlatformInitRuntimeDone event indicates that the function initialization phase has completed.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#platform-initRuntimeDone
type RecordPlatformInitRuntimeDone struct {
	InitType lambdaext.InitType `json:"initializationType"`
	Phase    Phase              `json:"phase"`
	Status   Status             `json:"status"`
	Spans    []Span             `json:"spans,omitempty"`
}

// RecordPlatformInitReport event contains an overall report of the function initialization phase.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#platform-initReport
type RecordPlatformInitReport struct {
	InitType lambdaext.InitType `json:"initializationType"`
	Phase    Phase              `json:"phase"`
	Metrics  InitReportMetrics  `json:"metrics"`
	Spans    []Span             `json:"spans,omitempty"`
}

// RecordPlatformStart event indicates that the function invocation phase has started.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#platform-start
type RecordPlatformStart struct {
	RequestID lambdaext.RequestID       `json:"requestId"`
	Version   lambdaext.FunctionVersion `json:"version,omitempty"`
	Tracing   TraceContext              `json:"tracing,omitempty"`
}

// RecordPlatformRuntimeDone event indicates that the function invocation phase has completed.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#platform-runtimeDone
type RecordPlatformRuntimeDone struct {
	RequestID lambdaext.RequestID `json:"requestId"`
	Status    Status              `json:"status"`
	Metrics   RuntimeDoneMetrics  `json:"metrics,omitempty"`
	Tracing   TraceContext        `json:"tracing,omitempty"`
	Spans     []Span              `json:"spans,omitempty"`
}

// RecordPlatformReport event contains an overall report of the function completed phase.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#platform-report
type RecordPlatformReport struct {
	RequestID lambdaext.RequestID `json:"requestId"`
	Status    Status              `json:"status"`
	Metrics   ReportMetrics       `json:"metrics"`
	Tracing   TraceContext        `json:"tracing,omitempty"`
	Spans     []Span              `json:"spans,omitempty"`
}

// RecordPlatformExtension is generated when an extension registers with the extensions API.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#platform-extension
type RecordPlatformExtension struct {
	Name   lambdaext.ExtensionName `json:"name"`
	State  string                  `json:"state"`
	Events []extapi.EventType      `json:"events"`
}

// RecordPlatformTelemetrySubscription event contains information about an extension subscription.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#platform-telemetrySubscription
type RecordPlatformTelemetrySubscription struct {
	Name  lambdaext.ExtensionName            `json:"name"`
	State string                             `json:"state"`
	Types []extapi.TelemetrySubscriptionType `json:"types"`
}

// RecordPlatformLogsDropped event contains information about dropped events.
// Lambda emits the platform.logsDropped event when an extension can't process one or more events.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#platform-logsDropped
type RecordPlatformLogsDropped struct {
	DroppedBytes   int    `json:"droppedBytes"`
	DroppedRecords int    `json:"droppedRecords"`
	Reason         string `json:"reason"`
}

// RecordFunction event contains logs from the function code.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#telemetry-api-function
type RecordFunction string

// RecordExtension event contains logs from the extension code.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#telemetry-api-extension
type RecordExtension string

// Phase describes the phase when the initialization step occurs.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#InitPhase
type Phase string

const (
	// PhaseInit is a Phase when Lambda runs the function initialization in most cases.
	PhaseInit Phase = "init"
	// PhaseInvoke is a Phase when Lambda may re-run the function initialization code during the invoke phase in some error cases. (This is called a suppressed init.)
	PhaseInvoke Phase = "invoke"
)

// Status describes the status of an initialization or invocation phase.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#Status
type Status string

const (
	StatusSuccess Status = "success"
	StatusFailure Status = "failure"
	StatusError   Status = "error"
)

type SpanName string

const (
	// SpanResponseLatency span describes how long it took your Lambda function to start sending the response.
	SpanResponseLatency SpanName = "responseLatency"
	// SpanResponseDuration span describes how long it took your Lambda function to finish sending the entire response.
	SpanResponseDuration SpanName = "responseDuration"
)

// Span represents a unit of work or operation in a trace.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#Span
type Span struct {
	Name     SpanName             `json:"name"`
	Start    time.Time            `json:"start"`
	Duration lambdaext.DurationMs `json:"durationMs"`
}

// InitReportMetrics contains metrics about an initialization phase.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#InitReportMetrics
type InitReportMetrics struct {
	Duration lambdaext.DurationMs `json:"durationMs"`
}

// TraceContext describes the properties of a trace.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#TraceContext
type TraceContext struct {
	SpanID string                 `json:"spanId,omitempty"`
	Type   lambdaext.TracingType  `json:"type"`
	Value  lambdaext.TracingValue `json:"value"`
}

// RuntimeDoneMetrics contains metrics about an invocation phase.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#RuntimeDoneMetrics
type RuntimeDoneMetrics struct {
	Duration      lambdaext.DurationMs `json:"durationMs"`
	ProducedBytes int                  `json:"producedBytes,omitempty"`
}

// ReportMetrics contains metrics about a completed phase.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#ReportMetrics
type ReportMetrics struct {
	BilledDuration  lambdaext.DurationMs `json:"billedDurationMs"`
	Duration        lambdaext.DurationMs `json:"durationMs"`
	InitDuration    lambdaext.DurationMs `json:"initDurationMs,omitempty"`
	MaxMemoryUsedMB int                  `json:"maxMemoryUsedMB"`
	MemorySizeMB    int                  `json:"memorySizeMB"`
	RestoreDuration lambdaext.DurationMs `json:"restoreDurationMs,omitempty"`
}

// Decode consumes all logs from json array stream and send them to the provided channel.
// Decode is low-level function. Consider using Run instead and implement TelemetryProcessor.
// Decode drains and closes the input stream afterwards.
func Decode(ctx context.Context, r io.ReadCloser, logs chan<- Event) error {
	return internal.Decode(ctx, r, logs, decodeNext)
}

func decodeNext(d *json.Decoder) (Event, error) {
	msg := Event{}
	if err := d.Decode(&msg); err != nil {
		return msg, fmt.Errorf("could not decode log message from json array: %w", err)
	}
	var unmarshalErr error
	switch msg.Type {
	case TypePlatformInitStart:
		record := RecordPlatformInitStart{}
		unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
		msg.Record = record
	case TypePlatformInitRuntimeDone:
		record := RecordPlatformInitRuntimeDone{}
		unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
		msg.Record = record
	case TypePlatformInitReport:
		record := RecordPlatformInitReport{}
		unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
		msg.Record = record
	case TypePlatformStart:
		record := RecordPlatformStart{}
		unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
		msg.Record = record
	case TypePlatformRuntimeDone:
		record := RecordPlatformRuntimeDone{}
		unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
		msg.Record = record
	case TypePlatformReport:
		record := RecordPlatformReport{}
		unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
		msg.Record = record
	case TypePlatformExtension:
		record := RecordPlatformExtension{}
		unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
		msg.Record = record
	case TypePlatformTelemetrySubscription:
		record := RecordPlatformTelemetrySubscription{}
		unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
		msg.Record = record
	case TypePlatformLogsDropped:
		record := RecordPlatformLogsDropped{}
		unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
		msg.Record = record
	case TypeFunction:
		record := RecordFunction("")
		unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
		msg.Record = record
	case TypeExtension:
		record := RecordExtension("")
		unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
		msg.Record = record
	default:
		return msg, fmt.Errorf(`could not decode unknown event type "%s" and record "%s"`, msg.Type, msg.RawRecord)
	}
	if unmarshalErr != nil {
		return msg, fmt.Errorf("could not decode log record %s for event type %s with error: %w", msg.RawRecord, msg.Type, unmarshalErr)
	}

	return msg, nil
}

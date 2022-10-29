// Sample extension to demonstrate how to use log fields and convert them into OpenTelemetry metrics.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/go-logr/stdr"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/logsapi"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/metric/unit"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

type LogProcessor struct {
	sdk *metric.MeterProvider

	duration           syncfloat64.Histogram
	billedDuration     syncfloat64.Histogram
	initDuration       syncfloat64.Histogram
	memorySizeMB       syncint64.Histogram
	maxMemoryUsedMB    syncint64.Histogram
	platformFaults     syncint64.Counter
	runtimeDone        syncint64.Counter
	logsDroppedBytes   syncint64.Counter
	logsDroppedRecords syncint64.Counter
}

func (lp *LogProcessor) Init(ctx context.Context, client *extapi.Client) error {
	exp, err := stdoutmetric.New(stdoutmetric.WithEncoder(json.NewEncoder(os.Stdout)))
	if err != nil {
		return err
	}

	lp.sdk = metric.NewMeterProvider(
		metric.WithResource(resource.NewSchemaless(
			semconv.CloudProviderAWS,
			semconv.CloudPlatformAWSLambda,
			semconv.CloudRegionKey.String(extapi.EnvAWSRegion()),
			semconv.FaaSNameKey.String(client.FunctionName()),
			semconv.FaaSVersionKey.String(client.FunctionVersion()),
			semconv.FaaSMaxMemoryKey.Int(extapi.EnvAWSLambdaFunctionMemorySizeMB()),
		)),
		metric.WithReader(metric.NewPeriodicReader(exp)),
	)

	meter := lp.sdk.Meter("lambda_function")

	lp.duration, err = meter.SyncFloat64().Histogram(
		"lambda_duration_ms",
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("the amount of time that your function's handler method spent processing the event"),
	)
	if err != nil {
		return err
	}
	lp.billedDuration, err = meter.SyncFloat64().Histogram(
		"lambda_duration_billed_ms",
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("the amount of time billed for the invocation"),
	)
	if err != nil {
		return err
	}
	lp.initDuration, err = meter.SyncFloat64().Histogram(
		"lambda_duration_init_ms",
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("for the first request served, the amount of time it took the runtime to load the function and run code outside of the handler method"),
	)
	if err != nil {
		return err
	}
	lp.memorySizeMB, err = meter.SyncInt64().Histogram(
		"lambda_memory_size_bytes",
		instrument.WithUnit(unit.Bytes),
		instrument.WithDescription("the amount of memory allocated to the function"),
	)
	if err != nil {
		return err
	}
	lp.maxMemoryUsedMB, err = meter.SyncInt64().Histogram(
		"lambda_max_memory_used_bytes",
		instrument.WithUnit(unit.Bytes),
		instrument.WithDescription("the amount of memory used by the function"),
	)
	if err != nil {
		return err
	}
	lp.platformFaults, err = meter.SyncInt64().Counter(
		"lambda_platform_faults",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("runtime or execution environment errors"),
	)
	if err != nil {
		return err
	}
	lp.runtimeDone, err = meter.SyncInt64().Counter(
		"lambda_runtime_done",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("function invocation completes either successfully or with an error"),
	)
	if err != nil {
		return err
	}
	lp.logsDroppedBytes, err = meter.SyncInt64().Counter(
		"lambda_logs_dropped_bytes",
		instrument.WithUnit(unit.Bytes),
		instrument.WithDescription("dropped bytes when an extension is not able to process the number of logs that it is receiving"),
	)
	if err != nil {
		return err
	}
	lp.logsDroppedRecords, err = meter.SyncInt64().Counter(
		"lambda_logs_dropped_records",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("dropped records when an extension is not able to process the number of logs that it is receiving"),
	)
	if err != nil {
		return err
	}

	return nil
}

func (lp *LogProcessor) Process(ctx context.Context, msg logsapi.Log) error {
	var err error
	switch record := msg.Record.(type) {
	case logsapi.RecordPlatformReport:
		lp.duration.Record(ctx, record.Metrics.DurationMs)
		lp.billedDuration.Record(ctx, record.Metrics.BilledDurationMs)
		lp.initDuration.Record(ctx, record.Metrics.InitDurationMs)
		lp.memorySizeMB.Record(ctx, int64(record.Metrics.MemorySizeMB*1024*1024))
		lp.maxMemoryUsedMB.Record(ctx, int64(record.Metrics.MaxMemoryUsedMB*1024*1024))
	case logsapi.RecordPlatformFault:
		lp.platformFaults.Add(ctx, 1)
	case logsapi.RecordPlatformRuntimeDone:
		lp.runtimeDone.Add(ctx, 1, attribute.String("status", string(record.Status)))

		// RecordPlatformRuntimeDone is generated after the function invocation completes either successfully or with an error.
		// The extension can use this message to stop all the telemetry collection for this function invocation.
		err = lp.sdk.ForceFlush(ctx)

	case logsapi.RecordPlatformLogsDropped:
		lp.logsDroppedBytes.Add(ctx, int64(record.DroppedBytes))
		lp.logsDroppedBytes.Add(ctx, int64(record.DroppedRecords))
	}

	return err
}

func (lp *LogProcessor) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	return lp.sdk.Shutdown(ctx)
}

func main() {
	// log library debug messages
	stdr.SetVerbosity(1)
	logger := stdr.New(log.New(os.Stdout, "", log.Lshortfile))

	if err := logsapi.Run(
		context.Background(),
		&LogProcessor{},
		logsapi.WithLogger(logger),
		logsapi.WithBufferingCfg(&extapi.LogsBufferingCfg{TimeoutMS: 25, MaxBytes: 262144, MaxItems: 1000}),
	); err != nil {
		log.Panic(err)
	}
}
package otel_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	lambdaext "github.com/zakharovvi/aws-lambda-extensions"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

var registerResp = &extapi.RegisterResponse{
	FunctionName:    "test-name",
	FunctionVersion: "$LATEST",
	Handler:         "main",
	AccountID:       "0123456789",
}

func TestEventTriplet_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		triplet otel.EventTriplet
		want    bool
	}{
		{
			"init",
			otel.EventTriplet{
				Type:        telemetryapi.PhaseInit,
				Start:       telemetryapi.Event{Type: telemetryapi.TypePlatformInitStart},
				RuntimeDone: telemetryapi.Event{Type: telemetryapi.TypePlatformInitRuntimeDone},
				Report:      telemetryapi.Event{Type: telemetryapi.TypePlatformInitReport},
			},
			true,
		},
		{
			"invoke",
			otel.EventTriplet{
				Type:        telemetryapi.PhaseInvoke,
				Start:       telemetryapi.Event{Type: telemetryapi.TypePlatformStart},
				RuntimeDone: telemetryapi.Event{Type: telemetryapi.TypePlatformRuntimeDone},
				Report:      telemetryapi.Event{Type: telemetryapi.TypePlatformReport},
			},
			true,
		},
		{
			"unknown type",
			otel.EventTriplet{
				Type:        "unknown type",
				Start:       telemetryapi.Event{Type: telemetryapi.TypePlatformInitStart},
				RuntimeDone: telemetryapi.Event{Type: telemetryapi.TypePlatformInitRuntimeDone},
				Report:      telemetryapi.Event{Type: telemetryapi.TypePlatformInitReport},
			},
			false,
		},
		{
			"mismatched events",
			otel.EventTriplet{
				Type:        telemetryapi.PhaseInit,
				Start:       telemetryapi.Event{Type: telemetryapi.TypePlatformInitStart},
				RuntimeDone: telemetryapi.Event{Type: telemetryapi.TypePlatformInitRuntimeDone},
				Report:      telemetryapi.Event{Type: telemetryapi.TypePlatformReport},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.triplet.IsValid())
		})
	}
}

func TestSpanConverter_ConvertIntoSpans_TracingNotEnabled(t *testing.T) {
	t.Parallel()

	sc := otel.NewSpanConverter(context.Background(), registerResp)

	triplet := getInvokeTriplet()
	record := triplet.Start.Record.(telemetryapi.RecordPlatformStart)
	record.Tracing = telemetryapi.TraceContext{}
	triplet.Start.Record = record

	spans, _, err := sc.ConvertIntoSpans(triplet)
	require.NoError(t, err)
	require.False(t, spans[2].Parent().TraceID().IsValid())
}

func TestSpanConverter_ConvertIntoSpans_SpanContext(t *testing.T) {
	t.Parallel()

	sc := otel.NewSpanConverter(context.Background(), registerResp)

	triplet := getInvokeTriplet()
	spans, spanContext, _ := sc.ConvertIntoSpans(triplet)
	require.Equal(t, spans[2].SpanContext(), spanContext)
}

func TestSpanConverter_ConvertIntoSpans(t *testing.T) {
	t.Setenv("AWS_REGION", "eu-west-1")
	t.Setenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE", "128")

	tests := []struct {
		name     string
		triplet  otel.EventTriplet
		wantJSON string
	}{
		{
			name:     "init",
			triplet:  getInitTriplet(),
			wantJSON: wantInitJSON,
		},
		{
			name:     "invoke",
			triplet:  getInvokeTriplet(),
			wantJSON: wantInvokeJSON,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := otel.NewSpanConverter(context.Background(), registerResp)

			gotSpans, _, err := sc.ConvertIntoSpans(tt.triplet)
			require.NoError(t, err)

			// span IDs generated randomly. Replace them for string comparison
			stubs := tracetest.SpanStubsFromReadOnlySpans(gotSpans)
			for i, stub := range stubs {
				if stub.Name == "test-name/responseLatency" {
					stubs[i].SpanContext = stub.SpanContext.WithSpanID([8]byte{1})
				}
				if stub.Name == "test-name/responseDuration" {
					stubs[i].SpanContext = stub.SpanContext.WithSpanID([8]byte{2})
				}
				if stub.Name == "test-name/init" {
					stubs[i].SpanContext = stub.SpanContext.WithTraceID([16]byte{3}).WithSpanID([8]byte{4})
				}
			}

			var b bytes.Buffer
			exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint(), stdouttrace.WithWriter(&b))
			require.NoError(t, err)

			err = exporter.ExportSpans(context.Background(), stubs.Snapshots())
			require.NoError(t, err)
			got := b.String()

			require.Equal(t, tt.wantJSON, got)
		})
	}
}

var wantInvokeJSON = `{
	"Name": "test-name/responseLatency",
	"SpanContext": {
		"TraceID": "637e16f01fbed7cb2ea0e5d7537a6258",
		"SpanID": "0100000000000000",
		"TraceFlags": "01",
		"TraceState": "",
		"Remote": false
	},
	"Parent": {
		"TraceID": "637e16f01fbed7cb2ea0e5d7537a6258",
		"SpanID": "7cd833ab5300d004",
		"TraceFlags": "01",
		"TraceState": "",
		"Remote": false
	},
	"SpanKind": 2,
	"StartTime": "2022-11-23T12:49:53.086Z",
	"EndTime": "2022-11-23T12:49:53.087Z",
	"Attributes": null,
	"Events": [],
	"Links": [],
	"Status": {
		"Code": "Unset",
		"Description": ""
	},
	"DroppedAttributes": 0,
	"DroppedEvents": 0,
	"DroppedLinks": 0,
	"ChildSpanCount": 0,
	"Resource": [
		{
			"Key": "cloud.account.id",
			"Value": {
				"Type": "STRING",
				"Value": "0123456789"
			}
		},
		{
			"Key": "cloud.platform",
			"Value": {
				"Type": "STRING",
				"Value": "aws_lambda"
			}
		},
		{
			"Key": "cloud.provider",
			"Value": {
				"Type": "STRING",
				"Value": "aws"
			}
		},
		{
			"Key": "cloud.region",
			"Value": {
				"Type": "STRING",
				"Value": "eu-west-1"
			}
		},
		{
			"Key": "faas.max_memory",
			"Value": {
				"Type": "INT64",
				"Value": 128
			}
		},
		{
			"Key": "faas.name",
			"Value": {
				"Type": "STRING",
				"Value": "test-name"
			}
		},
		{
			"Key": "faas.version",
			"Value": {
				"Type": "STRING",
				"Value": "$LATEST"
			}
		}
	],
	"InstrumentationLibrary": {
		"Name": "github.com/zakharovvi/aws-lambda-extensions/telemetryapi/otel",
		"Version": "",
		"SchemaURL": ""
	}
}
{
	"Name": "test-name/responseDuration",
	"SpanContext": {
		"TraceID": "637e16f01fbed7cb2ea0e5d7537a6258",
		"SpanID": "0200000000000000",
		"TraceFlags": "01",
		"TraceState": "",
		"Remote": false
	},
	"Parent": {
		"TraceID": "637e16f01fbed7cb2ea0e5d7537a6258",
		"SpanID": "7cd833ab5300d004",
		"TraceFlags": "01",
		"TraceState": "",
		"Remote": false
	},
	"SpanKind": 2,
	"StartTime": "2022-11-23T12:49:53.233Z",
	"EndTime": "2022-11-23T12:49:53.2552Z",
	"Attributes": null,
	"Events": [],
	"Links": [],
	"Status": {
		"Code": "Unset",
		"Description": ""
	},
	"DroppedAttributes": 0,
	"DroppedEvents": 0,
	"DroppedLinks": 0,
	"ChildSpanCount": 0,
	"Resource": [
		{
			"Key": "cloud.account.id",
			"Value": {
				"Type": "STRING",
				"Value": "0123456789"
			}
		},
		{
			"Key": "cloud.platform",
			"Value": {
				"Type": "STRING",
				"Value": "aws_lambda"
			}
		},
		{
			"Key": "cloud.provider",
			"Value": {
				"Type": "STRING",
				"Value": "aws"
			}
		},
		{
			"Key": "cloud.region",
			"Value": {
				"Type": "STRING",
				"Value": "eu-west-1"
			}
		},
		{
			"Key": "faas.max_memory",
			"Value": {
				"Type": "INT64",
				"Value": 128
			}
		},
		{
			"Key": "faas.name",
			"Value": {
				"Type": "STRING",
				"Value": "test-name"
			}
		},
		{
			"Key": "faas.version",
			"Value": {
				"Type": "STRING",
				"Value": "$LATEST"
			}
		}
	],
	"InstrumentationLibrary": {
		"Name": "github.com/zakharovvi/aws-lambda-extensions/telemetryapi/otel",
		"Version": "",
		"SchemaURL": ""
	}
}
{
	"Name": "test-name/invoke",
	"SpanContext": {
		"TraceID": "637e16f01fbed7cb2ea0e5d7537a6258",
		"SpanID": "7cd833ab5300d004",
		"TraceFlags": "01",
		"TraceState": "",
		"Remote": false
	},
	"Parent": {
		"TraceID": "637e16f01fbed7cb2ea0e5d7537a6258",
		"SpanID": "5ac36eec7a279fc5",
		"TraceFlags": "01",
		"TraceState": "",
		"Remote": true
	},
	"SpanKind": 2,
	"StartTime": "2022-11-23T12:49:53.086Z",
	"EndTime": "2022-11-23T12:49:53.258Z",
	"Attributes": [
		{
			"Key": "faas.execution",
			"Value": {
				"Type": "STRING",
				"Value": "cfa3c5e3-4441-42cc-86d0-404768d42e1b"
			}
		},
		{
			"Key": "aws.lambda.produced_bytes",
			"Value": {
				"Type": "INT64",
				"Value": 16
			}
		},
		{
			"Key": "aws.lambda.memory_size_mb",
			"Value": {
				"Type": "INT64",
				"Value": 128
			}
		},
		{
			"Key": "aws.lambda.max_memory_used_mb",
			"Value": {
				"Type": "INT64",
				"Value": 84
			}
		},
		{
			"Key": "aws.lambda.billed_duration_ms",
			"Value": {
				"Type": "INT64",
				"Value": 694
			}
		}
	],
	"Events": [],
	"Links": [
		{
			"SpanContext": {
				"TraceID": "01000000000000000000000000000000",
				"SpanID": "0200000000000000",
				"TraceFlags": "00",
				"TraceState": "",
				"Remote": true
			},
			"Attributes": [
				{
					"Key": "aws.lambda.link_type",
					"Value": {
						"Type": "STRING",
						"Value": "previous-trace"
					}
				}
			],
			"DroppedAttributeCount": 0
		}
	],
	"Status": {
		"Code": "Ok",
		"Description": ""
	},
	"DroppedAttributes": 0,
	"DroppedEvents": 0,
	"DroppedLinks": 0,
	"ChildSpanCount": 2,
	"Resource": [
		{
			"Key": "cloud.account.id",
			"Value": {
				"Type": "STRING",
				"Value": "0123456789"
			}
		},
		{
			"Key": "cloud.platform",
			"Value": {
				"Type": "STRING",
				"Value": "aws_lambda"
			}
		},
		{
			"Key": "cloud.provider",
			"Value": {
				"Type": "STRING",
				"Value": "aws"
			}
		},
		{
			"Key": "cloud.region",
			"Value": {
				"Type": "STRING",
				"Value": "eu-west-1"
			}
		},
		{
			"Key": "faas.max_memory",
			"Value": {
				"Type": "INT64",
				"Value": 128
			}
		},
		{
			"Key": "faas.name",
			"Value": {
				"Type": "STRING",
				"Value": "test-name"
			}
		},
		{
			"Key": "faas.version",
			"Value": {
				"Type": "STRING",
				"Value": "$LATEST"
			}
		}
	],
	"InstrumentationLibrary": {
		"Name": "github.com/zakharovvi/aws-lambda-extensions/telemetryapi/otel",
		"Version": "",
		"SchemaURL": ""
	}
}
`

func getInvokeTriplet() otel.EventTriplet {
	requestID := lambdaext.RequestID("cfa3c5e3-4441-42cc-86d0-404768d42e1b")

	return otel.EventTriplet{
		Type: telemetryapi.PhaseInvoke,
		Start: telemetryapi.Event{
			Type: telemetryapi.TypePlatformStart,
			Time: time.Date(2022, 11, 23, 12, 49, 53, int(86*time.Millisecond), time.UTC),
			Record: telemetryapi.RecordPlatformStart{
				RequestID: requestID,
				Version:   "$LATEST",
				Tracing: telemetryapi.TraceContext{
					SpanID: "7cd833ab5300d004",
					Type:   "X-Amzn-Trace-Id",
					Value:  "Root=1-637e16f0-1fbed7cb2ea0e5d7537a6258;Parent=5ac36eec7a279fc5;Sampled=1",
				},
			},
		},
		RuntimeDone: telemetryapi.Event{
			Type: telemetryapi.TypePlatformRuntimeDone,
			Time: time.Date(2022, 11, 23, 12, 49, 53, int(256*time.Millisecond), time.UTC),
			Record: telemetryapi.RecordPlatformRuntimeDone{
				RequestID: requestID,
				Status:    telemetryapi.StatusSuccess,
				Metrics: telemetryapi.RuntimeDoneMetrics{
					ProducedBytes: 16,
				},
				Spans: []telemetryapi.Span{
					{
						telemetryapi.SpanResponseLatency,
						time.Date(2022, 11, 23, 12, 49, 53, int(86*time.Millisecond), time.UTC),
						lambdaext.DurationMs(time.Millisecond),
					},
					{
						telemetryapi.SpanResponseDuration,
						time.Date(2022, 11, 23, 12, 49, 53, int(233*time.Millisecond), time.UTC),
						lambdaext.DurationMs(22200 * time.Microsecond),
					},
				},
			},
		},
		Report: telemetryapi.Event{
			Type: telemetryapi.TypePlatformReport,
			Time: time.Date(2022, 11, 23, 12, 49, 53, int(258*time.Millisecond), time.UTC),
			Record: telemetryapi.RecordPlatformReport{
				RequestID: requestID,
				Status:    telemetryapi.StatusSuccess,
				Metrics: telemetryapi.ReportMetrics{
					BilledDuration:  lambdaext.DurationMs(694 * time.Millisecond),
					Duration:        lambdaext.DurationMs(693920 * time.Microsecond),
					InitDuration:    lambdaext.DurationMs(397680 * time.Microsecond),
					MaxMemoryUsedMB: 84,
					MemorySizeMB:    128,
					RestoreDuration: lambdaext.DurationMs(123450 * time.Microsecond),
				},
				Tracing: telemetryapi.TraceContext{},
			},
		},
		PrevSC: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    [16]byte{1},
			SpanID:     [8]byte{2},
			TraceFlags: 0,
			Remote:     true,
		}),
	}
}

var wantInitJSON = `{
	"Name": "test-name/init",
	"SpanContext": {
		"TraceID": "03000000000000000000000000000000",
		"SpanID": "0400000000000000",
		"TraceFlags": "01",
		"TraceState": "",
		"Remote": false
	},
	"Parent": {
		"TraceID": "00000000000000000000000000000000",
		"SpanID": "0000000000000000",
		"TraceFlags": "00",
		"TraceState": "",
		"Remote": false
	},
	"SpanKind": 2,
	"StartTime": "2022-11-23T12:49:53.086Z",
	"EndTime": "2022-11-23T12:49:53.258Z",
	"Attributes": [
		{
			"Key": "faas.coldstart",
			"Value": {
				"Type": "BOOL",
				"Value": true
			}
		},
		{
			"Key": "aws.lambda.runtime_version",
			"Value": {
				"Type": "STRING",
				"Value": "nodejs-14.v3"
			}
		},
		{
			"Key": "aws.lambda.runtime_version_arn",
			"Value": {
				"Type": "STRING",
				"Value": "arn"
			}
		}
	],
	"Events": [],
	"Links": [],
	"Status": {
		"Code": "Error",
		"Description": "init-error"
	},
	"DroppedAttributes": 0,
	"DroppedEvents": 0,
	"DroppedLinks": 0,
	"ChildSpanCount": 0,
	"Resource": [
		{
			"Key": "cloud.account.id",
			"Value": {
				"Type": "STRING",
				"Value": "0123456789"
			}
		},
		{
			"Key": "cloud.platform",
			"Value": {
				"Type": "STRING",
				"Value": "aws_lambda"
			}
		},
		{
			"Key": "cloud.provider",
			"Value": {
				"Type": "STRING",
				"Value": "aws"
			}
		},
		{
			"Key": "cloud.region",
			"Value": {
				"Type": "STRING",
				"Value": "eu-west-1"
			}
		},
		{
			"Key": "faas.max_memory",
			"Value": {
				"Type": "INT64",
				"Value": 128
			}
		},
		{
			"Key": "faas.name",
			"Value": {
				"Type": "STRING",
				"Value": "test-name"
			}
		},
		{
			"Key": "faas.version",
			"Value": {
				"Type": "STRING",
				"Value": "$LATEST"
			}
		}
	],
	"InstrumentationLibrary": {
		"Name": "github.com/zakharovvi/aws-lambda-extensions/telemetryapi/otel",
		"Version": "",
		"SchemaURL": ""
	}
}
`

func getInitTriplet() otel.EventTriplet {
	return otel.EventTriplet{
		Type: telemetryapi.PhaseInit,
		Start: telemetryapi.Event{
			Type: telemetryapi.TypePlatformInitStart,
			Time: time.Date(2022, 11, 23, 12, 49, 53, int(86*time.Millisecond), time.UTC),
			Record: telemetryapi.RecordPlatformInitStart{
				InitType:          lambdaext.InitTypeOnDemand,
				Phase:             telemetryapi.PhaseInit,
				RuntimeVersion:    "nodejs-14.v3",
				RuntimeVersionARN: "arn",
			},
		},
		RuntimeDone: telemetryapi.Event{
			Type: telemetryapi.TypePlatformInitRuntimeDone,
			Time: time.Date(2022, 11, 23, 12, 49, 53, int(256*time.Millisecond), time.UTC),
			Record: telemetryapi.RecordPlatformInitRuntimeDone{
				InitType:  lambdaext.InitTypeOnDemand,
				Phase:     telemetryapi.PhaseInit,
				Status:    telemetryapi.StatusError,
				ErrorType: "init-error",
			},
		},
		Report: telemetryapi.Event{
			Type: telemetryapi.TypePlatformInitReport,
			Time: time.Date(2022, 11, 23, 12, 49, 53, int(258*time.Millisecond), time.UTC),
			Record: telemetryapi.RecordPlatformInitReport{
				InitType: lambdaext.InitTypeOnDemand,
				Phase:    telemetryapi.PhaseInit,
				Metrics: telemetryapi.InitReportMetrics{
					Duration: lambdaext.DurationMs(125330 * time.Microsecond),
				},
			},
		},
	}
}

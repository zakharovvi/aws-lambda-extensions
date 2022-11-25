package telemetryapi_test

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	lambdaext "github.com/zakharovvi/aws-lambda-extensions"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi"
)

func TestDecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		response          string
		wantErrorContains string
		want              []telemetryapi.Event
	}{
		{
			name: "multiple messages",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.start",
					"record": {"requestId": "6f7f0961f83442118a7af6fe80b88d56"}
				},
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.logsDropped",
					"record": {"droppedBytes": 2, "droppedRecords": 3, "reason": "error"}
				}
			]`,
			wantErrorContains: "",
			want: []telemetryapi.Event{
				{
					Type:      telemetryapi.TypePlatformStart,
					Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
					RawRecord: json.RawMessage(`{"requestId": "6f7f0961f83442118a7af6fe80b88d56"}`),
					Record: telemetryapi.RecordPlatformStart{
						RequestID: "6f7f0961f83442118a7af6fe80b88d56",
						Tracing:   telemetryapi.TraceContext{},
					},
				},
				{
					Type:      telemetryapi.TypePlatformLogsDropped,
					Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
					RawRecord: json.RawMessage(`{"droppedBytes": 2, "droppedRecords": 3, "reason": "error"}`),
					Record: telemetryapi.RecordPlatformLogsDropped{
						DroppedBytes:   2,
						DroppedRecords: 3,
						Reason:         "error",
					},
				},
			},
		},
		{
			name: "unknown event type",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "unknown",
					"record": {"requestId": "6f7f0961f83442118a7af6fe80b88d56"}
				},
			]`,
			wantErrorContains: "unknown event type",
			want:              nil,
		},
		{
			name: "invalid json",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.start",
					"record": {"requestId": "6f7f0961f83442118a7af6fe80b88d56"}
				},
				{ INVALID_JSON
			]`,
			wantErrorContains: "invalid character",
			want: []telemetryapi.Event{
				{
					Type:      telemetryapi.TypePlatformStart,
					Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
					RawRecord: json.RawMessage(`{"requestId": "6f7f0961f83442118a7af6fe80b88d56"}`),
					Record: telemetryapi.RecordPlatformStart{
						RequestID: "6f7f0961f83442118a7af6fe80b88d56",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			eventsCh := make(chan telemetryapi.Event, 100)
			r := io.NopCloser(strings.NewReader(tt.response))
			err := telemetryapi.Decode(context.Background(), r, eventsCh)
			if tt.wantErrorContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErrorContains)
			}
			close(eventsCh)

			var events []telemetryapi.Event
			for event := range eventsCh {
				events = append(events, event)
			}
			require.Equal(t, tt.want, events)

			// check that body was drained and can be reused
			n, err := io.Copy(io.Discard, r)
			require.NoError(t, err)
			require.Zero(t, n)
		})
	}
}

func TestDecode_EventTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response string
		want     telemetryapi.Event
	}{
		{
			name: "platform.initStart",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.initStart",
					"record": {
						"initializationType": "on-demand",
						"phase": "init",
						"runtimeVersion": "nodejs-14.v3",
						"runtimeVersionArn": "arn"
					}
				}
			]`,
			want: telemetryapi.Event{
				Type: telemetryapi.TypePlatformInitStart,
				Time: time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"initializationType": "on-demand",
						"phase": "init",
						"runtimeVersion": "nodejs-14.v3",
						"runtimeVersionArn": "arn"
				}`),
				Record: telemetryapi.RecordPlatformInitStart{
					InitType:          lambdaext.InitTypeOnDemand,
					Phase:             telemetryapi.PhaseInit,
					RuntimeVersion:    "nodejs-14.v3",
					RuntimeVersionARN: "arn",
				},
			},
		},
		{
			name: "platform.initRuntimeDone",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.initRuntimeDone",
					"record": {
						"initializationType": "on-demand",
						"phase": "init",
						"status": "success"
					}
				}
			]`,
			want: telemetryapi.Event{
				Type: telemetryapi.TypePlatformInitRuntimeDone,
				Time: time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"initializationType": "on-demand",
						"phase": "init",
						"status": "success"
				}`),
				Record: telemetryapi.RecordPlatformInitRuntimeDone{
					InitType: lambdaext.InitTypeOnDemand,
					Phase:    telemetryapi.PhaseInit,
					Status:   telemetryapi.StatusSuccess,
				},
			},
		},
		{
			name: "platform.initReport",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.initReport",
					"record": {
						"initializationType": "on-demand",
						"phase": "init",
						"metrics": {
							"durationMs": 125.33
						}
					}
				}
			]`,
			want: telemetryapi.Event{
				Type: telemetryapi.TypePlatformInitReport,
				Time: time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"initializationType": "on-demand",
						"phase": "init",
						"metrics": {
							"durationMs": 125.33
						}
				}`),
				Record: telemetryapi.RecordPlatformInitReport{
					InitType: lambdaext.InitTypeOnDemand,
					Phase:    telemetryapi.PhaseInit,
					Metrics: telemetryapi.InitReportMetrics{
						Duration: lambdaext.DurationMs(125330 * time.Microsecond),
					},
				},
			},
		},
		{
			name: "platform.start",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.start",
					"record": {
						"requestId": "6d68ca91-49c9-448d-89b8-7ca3e6dc66aa",
						"version": "$LATEST",
						"tracing": {
							"spanId": "54565fb41ac79632",
							"type": "X-Amzn-Trace-Id",
							"value": "Root=1-62e900b2-710d76f009d6e7785905449a;Parent=0efbd19962d95b05;Sampled=1"
						}
					}
				}
			]`,
			want: telemetryapi.Event{
				Type: telemetryapi.TypePlatformStart,
				Time: time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"requestId": "6d68ca91-49c9-448d-89b8-7ca3e6dc66aa",
						"version": "$LATEST",
						"tracing": {
							"spanId": "54565fb41ac79632",
							"type": "X-Amzn-Trace-Id",
							"value": "Root=1-62e900b2-710d76f009d6e7785905449a;Parent=0efbd19962d95b05;Sampled=1"
						}
				}`),
				Record: telemetryapi.RecordPlatformStart{
					RequestID: "6d68ca91-49c9-448d-89b8-7ca3e6dc66aa",
					Version:   "$LATEST",
					Tracing: telemetryapi.TraceContext{
						SpanID: "54565fb41ac79632",
						Type:   lambdaext.TracingTypeAWSXRay,
						Value:  lambdaext.TracingValue("Root=1-62e900b2-710d76f009d6e7785905449a;Parent=0efbd19962d95b05;Sampled=1"),
					},
				},
			},
		},
		{
			name: "platform.runtimeDone",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.runtimeDone",
					"record": {
						"requestId": "6d68ca91-49c9-448d-89b8-7ca3e6dc66aa",
						"status": "success",
						"metrics": {
							"durationMs": 140.0,
							"producedBytes": 16
						},
						"tracing": {
							"spanId": "54565fb41ac79632",
							"type": "X-Amzn-Trace-Id",
							"value": "Root=1-62e900b2-710d76f009d6e7785905449a;Parent=0efbd19962d95b05;Sampled=1"
						},
						"spans": [
							{
								"name": "someTimeSpan",
								"start": "2020-08-20T12:31:32.0Z",
								"durationMs": 70.5
							}
						]
					}
				}
			]`,
			want: telemetryapi.Event{
				Type: telemetryapi.TypePlatformRuntimeDone,
				Time: time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"requestId": "6d68ca91-49c9-448d-89b8-7ca3e6dc66aa",
						"status": "success",
						"metrics": {
							"durationMs": 140.0,
							"producedBytes": 16
						},
						"tracing": {
							"spanId": "54565fb41ac79632",
							"type": "X-Amzn-Trace-Id",
							"value": "Root=1-62e900b2-710d76f009d6e7785905449a;Parent=0efbd19962d95b05;Sampled=1"
						},
						"spans": [
							{
								"name": "someTimeSpan",
								"start": "2020-08-20T12:31:32.0Z",
								"durationMs": 70.5
							}
						]
				}`),
				Record: telemetryapi.RecordPlatformRuntimeDone{
					RequestID: "6d68ca91-49c9-448d-89b8-7ca3e6dc66aa",
					Status:    telemetryapi.StatusSuccess,
					Metrics: telemetryapi.RuntimeDoneMetrics{
						Duration:      lambdaext.DurationMs(140 * time.Millisecond),
						ProducedBytes: 16,
					},
					Tracing: telemetryapi.TraceContext{
						SpanID: "54565fb41ac79632",
						Type:   lambdaext.TracingTypeAWSXRay,
						Value:  lambdaext.TracingValue("Root=1-62e900b2-710d76f009d6e7785905449a;Parent=0efbd19962d95b05;Sampled=1"),
					},
					Spans: []telemetryapi.Span{
						{
							Name:     "someTimeSpan",
							Start:    time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
							Duration: lambdaext.DurationMs(70500 * time.Microsecond),
						},
					},
				},
			},
		},
		{
			name: "platform.report",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.report",
					"record": {
						"requestId": "6d68ca91-49c9-448d-89b8-7ca3e6dc66aa",
						"status": "success",
						"metrics": {
							"billedDurationMs": 694,
							"durationMs": 693.92,
							"initDurationMs": 397.68,
							"maxMemoryUsedMB": 84,
							"memorySizeMB": 128,
							"restoreDurationMs": 123.45
						},
						"tracing": {
							"spanId": "54565fb41ac79632",
							"type": "X-Amzn-Trace-Id",
							"value": "Root=1-62e900b2-710d76f009d6e7785905449a;Parent=0efbd19962d95b05;Sampled=1"
						}
					}
				}
			]`,
			want: telemetryapi.Event{
				Type: telemetryapi.TypePlatformReport,
				Time: time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"requestId": "6d68ca91-49c9-448d-89b8-7ca3e6dc66aa",
						"status": "success",
						"metrics": {
							"billedDurationMs": 694,
							"durationMs": 693.92,
							"initDurationMs": 397.68,
							"maxMemoryUsedMB": 84,
							"memorySizeMB": 128,
							"restoreDurationMs": 123.45
						},
						"tracing": {
							"spanId": "54565fb41ac79632",
							"type": "X-Amzn-Trace-Id",
							"value": "Root=1-62e900b2-710d76f009d6e7785905449a;Parent=0efbd19962d95b05;Sampled=1"
						}
				}`),
				Record: telemetryapi.RecordPlatformReport{
					RequestID: "6d68ca91-49c9-448d-89b8-7ca3e6dc66aa",
					Status:    telemetryapi.StatusSuccess,
					Metrics: telemetryapi.ReportMetrics{
						BilledDuration:  lambdaext.DurationMs(694 * time.Millisecond),
						Duration:        lambdaext.DurationMs(693920 * time.Microsecond),
						InitDuration:    lambdaext.DurationMs(397680 * time.Microsecond),
						MaxMemoryUsedMB: 84,
						MemorySizeMB:    128,
						RestoreDuration: lambdaext.DurationMs(123450 * time.Microsecond),
					},
					Tracing: telemetryapi.TraceContext{
						SpanID: "54565fb41ac79632",
						Type:   lambdaext.TracingTypeAWSXRay,
						Value:  lambdaext.TracingValue("Root=1-62e900b2-710d76f009d6e7785905449a;Parent=0efbd19962d95b05;Sampled=1"),
					},
				},
			},
		},
		{
			name: "platform.extension",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.extension",
					"record": {
						"events": [ "INVOKE", "SHUTDOWN" ],
						"name": "my-telemetry-extension",
						"state": "Ready"
					}
				}
			]`,
			want: telemetryapi.Event{
				Type: telemetryapi.TypePlatformExtension,
				Time: time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"events": [ "INVOKE", "SHUTDOWN" ],
						"name": "my-telemetry-extension",
						"state": "Ready"
				}`),
				Record: telemetryapi.RecordPlatformExtension{
					Name:   "my-telemetry-extension",
					State:  "Ready",
					Events: []extapi.EventType{extapi.Invoke, extapi.Shutdown},
				},
			},
		},
		{
			name: "platform.telemetrySubscription",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.telemetrySubscription",
					"record": {
						"name": "my-telemetry-extension",
						"state": "Subscribed",
						"types": [ "platform", "function" ]
					}
				}
			]`,
			want: telemetryapi.Event{
				Type: telemetryapi.TypePlatformTelemetrySubscription,
				Time: time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"name": "my-telemetry-extension",
						"state": "Subscribed",
						"types": [ "platform", "function" ]
				}`),
				Record: telemetryapi.RecordPlatformTelemetrySubscription{
					Name:  "my-telemetry-extension",
					State: "Subscribed",
					Types: []extapi.TelemetrySubscriptionType{extapi.TelemetrySubscriptionTypePlatform, extapi.TelemetrySubscriptionTypeFunction},
				},
			},
		},
		{
			name: "platform.logsDropped",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.logsDropped",
					"record": {
						"droppedBytes": 12345,
						"droppedRecords": 123,
						"reason": "Consumer seems to have fallen behind as it has not acknowledged receipt of logs."
					}
				}
			]`,
			want: telemetryapi.Event{
				Type: telemetryapi.TypePlatformLogsDropped,
				Time: time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"droppedBytes": 12345,
						"droppedRecords": 123,
						"reason": "Consumer seems to have fallen behind as it has not acknowledged receipt of logs."
				}`),
				Record: telemetryapi.RecordPlatformLogsDropped{
					DroppedBytes:   12345,
					DroppedRecords: 123,
					Reason:         "Consumer seems to have fallen behind as it has not acknowledged receipt of logs.",
				},
			},
		},
		{
			name: "function",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "function",
					"record": "Hello from function"
				}
			]`,
			want: telemetryapi.Event{
				Type:      telemetryapi.TypeFunction,
				Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`"Hello from function"`),
				Record:    telemetryapi.RecordFunction("Hello from function"),
			},
		},
		{
			name: "extension",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "extension",
					"record": "Hello from extension"
				}
			]`,
			want: telemetryapi.Event{
				Type:      telemetryapi.TypeExtension,
				Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`"Hello from extension"`),
				Record:    telemetryapi.RecordExtension("Hello from extension"),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			events := make(chan telemetryapi.Event, 1)
			r := io.NopCloser(strings.NewReader(tt.response))
			err := telemetryapi.Decode(context.Background(), r, events)
			require.NoError(t, err)

			event := <-events
			require.Equal(t, tt.want.Time, event.Time)
			require.Equal(t, tt.want.Type, event.Type)
			require.JSONEq(t, string(tt.want.RawRecord), string(event.RawRecord))
			require.Equal(t, tt.want.Record, event.Record)
		})
	}
}

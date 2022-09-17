package logsapi_test

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zakharovvi/lambda-extensions/extapi"
	"github.com/zakharovvi/lambda-extensions/logsapi"
)

func TestDecodeLogs(t *testing.T) {
	tests := []struct {
		name              string
		response          string
		wantErrorContains string
		want              []logsapi.Log
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
					"type": "platform.end",
					"record": {"requestId": "6f7f0961f83442118a7af6fe80b88d56"}
				}
			]`,
			wantErrorContains: "",
			want: []logsapi.Log{
				{
					LogType:   logsapi.LogPlatformStart,
					Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
					RawRecord: json.RawMessage(`{"requestId": "6f7f0961f83442118a7af6fe80b88d56"}`),
					Record: &logsapi.RecordPlatformStart{
						RequestID: "6f7f0961f83442118a7af6fe80b88d56",
						Version:   "",
					},
				},
				{
					LogType:   logsapi.LogPlatformEnd,
					Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
					RawRecord: json.RawMessage(`{"requestId": "6f7f0961f83442118a7af6fe80b88d56"}`),
					Record: &logsapi.RecordPlatformEnd{
						RequestID: "6f7f0961f83442118a7af6fe80b88d56",
					},
				},
			},
		},
		{
			name: "unknown log event",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "unknown",
					"record": {"requestId": "6f7f0961f83442118a7af6fe80b88d56"}
				},
			]`,
			wantErrorContains: "unknown log type",
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
			want: []logsapi.Log{
				{
					LogType:   logsapi.LogPlatformStart,
					Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
					RawRecord: json.RawMessage(`{"requestId": "6f7f0961f83442118a7af6fe80b88d56"}`),
					Record: &logsapi.RecordPlatformStart{
						RequestID: "6f7f0961f83442118a7af6fe80b88d56",
						Version:   "",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logsCh := make(chan logsapi.Log, 100)
			r := io.NopCloser(strings.NewReader(tt.response))
			err := logsapi.DecodeLogs(r, logsCh)
			if tt.wantErrorContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErrorContains)
			}
			close(logsCh)

			var logs []logsapi.Log
			for log := range logsCh {
				logs = append(logs, log)
			}
			assert.Equal(t, tt.want, logs)

			// check that body was drained and can be reused
			n, err := io.Copy(io.Discard, r)
			assert.NoError(t, err)
			assert.Zero(t, n)
		})
	}
}

func TestDecodeLogs_LogTypes(t *testing.T) {
	platformFaultRecord := logsapi.RecordPlatformFault("RequestId: d783b35e-a91d-4251-af17-035953428a2c Process exited before completing request")
	functionRecord := logsapi.RecordFunction("Hello from function")
	extensionRecord := logsapi.RecordExtension("Hello from extension")

	tests := []struct {
		name     string
		response string
		want     logsapi.Log
	}{
		{
			name: "platform.start",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.start",
					"record": {"requestId": "6f7f0961f83442118a7af6fe80b88d56"}
				}
			]`,
			want: logsapi.Log{
				LogType:   logsapi.LogPlatformStart,
				Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{"requestId": "6f7f0961f83442118a7af6fe80b88d56"}`),
				Record: &logsapi.RecordPlatformStart{
					RequestID: "6f7f0961f83442118a7af6fe80b88d56",
					Version:   "",
				},
			},
		},
		{
			name: "platform.end",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.end",
					"record": {"requestId": "6f7f0961f83442118a7af6fe80b88d56"}
				}
			]`,
			want: logsapi.Log{
				LogType:   logsapi.LogPlatformEnd,
				Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{"requestId": "6f7f0961f83442118a7af6fe80b88d56"}`),
				Record: &logsapi.RecordPlatformEnd{
					RequestID: "6f7f0961f83442118a7af6fe80b88d56",
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
						"requestId": "6f7f0961f83442118a7af6fe80b88d56",
						"metrics": {
							"durationMs": 101.51,
							"billedDurationMs": 300,
							"memorySizeMB": 512,
							"maxMemoryUsedMB": 33,
							"initDurationMs": 116.67
						}
					}
				}
			]`,
			want: logsapi.Log{
				LogType: logsapi.LogPlatformReport,
				Time:    time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
					"requestId": "6f7f0961f83442118a7af6fe80b88d56",
					"metrics": {
						"durationMs": 101.51,
						"billedDurationMs": 300,
						"memorySizeMB": 512,
						"maxMemoryUsedMB": 33,
						"initDurationMs": 116.67
					}
				}`),
				Record: &logsapi.RecordPlatformReport{
					RequestID: "6f7f0961f83442118a7af6fe80b88d56",
					Metrics: logsapi.Metrics{
						DurationMs:       101.51,
						BilledDurationMs: 300,
						MemorySizeMB:     512,
						MaxMemoryUsedMB:  33,
						InitDurationMs:   116.67,
					},
				},
			},
		},
		{
			name: "platform.fault",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.fault",
					"record": "RequestId: d783b35e-a91d-4251-af17-035953428a2c Process exited before completing request"
				}
			]`,
			want: logsapi.Log{
				LogType:   logsapi.LogPlatformFault,
				Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`"RequestId: d783b35e-a91d-4251-af17-035953428a2c Process exited before completing request"`),
				Record:    &platformFaultRecord,
			},
		},
		{
			name: "platform.extension",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.extension",
					"record": {
						"name": "Foo.bar",
						"state": "Ready",
						"events": ["INVOKE", "SHUTDOWN"]
					 }
				}
			]`,
			want: logsapi.Log{
				LogType: logsapi.LogPlatformExtension,
				Time:    time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"name": "Foo.bar",
						"state": "Ready",
						"events": ["INVOKE", "SHUTDOWN"]
				 }`),
				Record: &logsapi.RecordPlatformExtension{
					Events: []extapi.EventType{extapi.Invoke, extapi.Shutdown},
					Name:   "Foo.bar",
					State:  "Ready",
				},
			},
		},
		{
			name: "platform.logsSubscription",
			response: `[
				{
					"time": "2020-08-20T12:31:32.0Z",
					"type": "platform.logsSubscription",
					"record": {
						"name": "Foo.bar",
						"state": "Subscribed",
						"types": ["function", "platform"]
					}
				}
			]`,
			want: logsapi.Log{
				LogType: logsapi.LogPlatformLogsSubscription,
				Time:    time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"name": "Foo.bar",
						"state": "Subscribed",
						"types": ["function", "platform"]
				}`),
				Record: &logsapi.RecordPlatformLogsSubscription{
					Name:  "Foo.bar",
					State: "Subscribed",
					Types: []extapi.LogSubscriptionType{extapi.Function, extapi.Platform},
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
						"reason": "Consumer seems to have fallen behind as it has not acknowledged receipt of logs.",
						"droppedRecords": 123,
						"droppedBytes": 12345
					}
				}
			]`,
			want: logsapi.Log{
				LogType: logsapi.LogPlatformLogsDropped,
				Time:    time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"reason": "Consumer seems to have fallen behind as it has not acknowledged receipt of logs.",
						"droppedRecords": 123,
						"droppedBytes": 12345
				}`),
				Record: &logsapi.RecordPlatformLogsDropped{
					DroppedBytes:   12345,
					DroppedRecords: 123,
					Reason:         "Consumer seems to have fallen behind as it has not acknowledged receipt of logs.",
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
					  "requestId":"6f7f0961f83442118a7af6fe80b88",
					  "status": "timeout"
				  }
				}
			]`,
			want: logsapi.Log{
				LogType: logsapi.LogPlatformRuntimeDone,
				Time:    time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
					  "requestId":"6f7f0961f83442118a7af6fe80b88",
					  "status": "timeout"
				}`),
				Record: &logsapi.RecordPlatformRuntimeDone{
					RequestID: "6f7f0961f83442118a7af6fe80b88",
					Status:    logsapi.RuntimeDoneTimeout,
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
			want: logsapi.Log{
				LogType:   logsapi.LogFunction,
				Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`"Hello from function"`),
				Record:    &functionRecord,
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
			want: logsapi.Log{
				LogType:   logsapi.LogExtension,
				Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`"Hello from extension"`),
				Record:    &extensionRecord,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := make(chan logsapi.Log, 1)
			r := io.NopCloser(strings.NewReader(tt.response))
			err := logsapi.DecodeLogs(r, logs)
			require.NoError(t, err)

			log := <-logs
			assert.Equal(t, tt.want.Time, log.Time)
			assert.Equal(t, tt.want.LogType, log.LogType)
			assert.JSONEq(t, string(tt.want.RawRecord), string(log.RawRecord))
			assert.Equal(t, tt.want.Record, log.Record)
		})
	}
}

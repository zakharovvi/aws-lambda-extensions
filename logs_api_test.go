package lambdaextensions_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zakharovvi/lambdaextensions"
)

const (
	logReceiverURL = "http://example.com:8080/logs"
)

func TestSubscribe(t *testing.T) {
	client, server, mux, err := register(t)
	require.NoError(t, err)
	defer server.Close()
	mux.HandleFunc("/2020-08-15/logs", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, testIdentifier, r.Header.Get("Lambda-Extension-Identifier"))

		req, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		subscribeReq := &lambdaextensions.SubscribeRequest{}
		assert.NoError(t, json.Unmarshal(req, subscribeReq))
		assert.Equal(t, logReceiverURL, subscribeReq.Destination.URI)
		assert.Equal(
			t,
			[]lambdaextensions.LogSubscriptionType{lambdaextensions.Platform, lambdaextensions.Function, lambdaextensions.Extension},
			subscribeReq.LogTypes,
		)

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Fatal(err)
		}
	})

	subscribeReq := lambdaextensions.NewSubscribeRequest(logReceiverURL, nil)
	err = client.Subscribe(context.Background(), subscribeReq)
	assert.NoError(t, err)
}

func TestDecodeLogs(t *testing.T) {
	tests := []struct {
		name              string
		response          string
		wantErrorContains string
		want              []lambdaextensions.Log
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
			want: []lambdaextensions.Log{
				{
					LogType:   lambdaextensions.LogPlatformStart,
					Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
					RawRecord: json.RawMessage(`{"requestId": "6f7f0961f83442118a7af6fe80b88d56"}`),
					Record: &lambdaextensions.PlatformStartRecord{
						RequestID: "6f7f0961f83442118a7af6fe80b88d56",
						Version:   "",
					},
				},
				{
					LogType:   lambdaextensions.LogPlatformEnd,
					Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
					RawRecord: json.RawMessage(`{"requestId": "6f7f0961f83442118a7af6fe80b88d56"}`),
					Record: &lambdaextensions.PlatformEndRecord{
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
			want: []lambdaextensions.Log{
				{
					LogType:   lambdaextensions.LogPlatformStart,
					Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
					RawRecord: json.RawMessage(`{"requestId": "6f7f0961f83442118a7af6fe80b88d56"}`),
					Record: &lambdaextensions.PlatformStartRecord{
						RequestID: "6f7f0961f83442118a7af6fe80b88d56",
						Version:   "",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logsCh := make(chan lambdaextensions.Log, 100)
			r := io.NopCloser(strings.NewReader(tt.response))
			err := lambdaextensions.DecodeLogs(r, logsCh)
			if tt.wantErrorContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErrorContains)
			}
			close(logsCh)

			var logs []lambdaextensions.Log
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
	platformFaultRecord := lambdaextensions.PlatformFaultRecord("RequestId: d783b35e-a91d-4251-af17-035953428a2c Process exited before completing request")
	functionRecord := lambdaextensions.FunctionRecord("Hello from function")
	extensionRecord := lambdaextensions.ExtensionRecord("Hello from extension")

	tests := []struct {
		name     string
		response string
		want     lambdaextensions.Log
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
			want: lambdaextensions.Log{
				LogType:   lambdaextensions.LogPlatformStart,
				Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{"requestId": "6f7f0961f83442118a7af6fe80b88d56"}`),
				Record: &lambdaextensions.PlatformStartRecord{
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
			want: lambdaextensions.Log{
				LogType:   lambdaextensions.LogPlatformEnd,
				Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{"requestId": "6f7f0961f83442118a7af6fe80b88d56"}`),
				Record: &lambdaextensions.PlatformEndRecord{
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
			want: lambdaextensions.Log{
				LogType: lambdaextensions.LogPlatformReport,
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
				Record: &lambdaextensions.PlatformReportRecord{
					RequestID: "6f7f0961f83442118a7af6fe80b88d56",
					Metrics: lambdaextensions.Metrics{
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
			want: lambdaextensions.Log{
				LogType:   lambdaextensions.LogPlatformFault,
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
			want: lambdaextensions.Log{
				LogType: lambdaextensions.LogPlatformExtension,
				Time:    time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"name": "Foo.bar",
						"state": "Ready",
						"events": ["INVOKE", "SHUTDOWN"]
				 }`),
				Record: &lambdaextensions.PlatformExtensionRecord{
					Events: []lambdaextensions.EventType{lambdaextensions.Invoke, lambdaextensions.Shutdown},
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
			want: lambdaextensions.Log{
				LogType: lambdaextensions.LogPlatformLogsSubscription,
				Time:    time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"name": "Foo.bar",
						"state": "Subscribed",
						"types": ["function", "platform"]
				}`),
				Record: &lambdaextensions.PlatformLogsSubscriptionRecord{
					Name:  "Foo.bar",
					State: "Subscribed",
					Types: []lambdaextensions.LogSubscriptionType{lambdaextensions.Function, lambdaextensions.Platform},
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
			want: lambdaextensions.Log{
				LogType: lambdaextensions.LogPlatformLogsDropped,
				Time:    time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
						"reason": "Consumer seems to have fallen behind as it has not acknowledged receipt of logs.",
						"droppedRecords": 123,
						"droppedBytes": 12345
				}`),
				Record: &lambdaextensions.PlatformLogsDroppedRecord{
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
			want: lambdaextensions.Log{
				LogType: lambdaextensions.LogPlatformRuntimeDone,
				Time:    time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`{
					  "requestId":"6f7f0961f83442118a7af6fe80b88",
					  "status": "timeout"
				}`),
				Record: &lambdaextensions.PlatformRuntimeDoneRecord{
					RequestID: "6f7f0961f83442118a7af6fe80b88",
					Status:    lambdaextensions.RuntimeDoneTimeout,
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
			want: lambdaextensions.Log{
				LogType:   lambdaextensions.LogFunction,
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
			want: lambdaextensions.Log{
				LogType:   lambdaextensions.LogExtension,
				Time:      time.Date(2020, 8, 20, 12, 31, 32, 0, time.UTC),
				RawRecord: json.RawMessage(`"Hello from extension"`),
				Record:    &extensionRecord,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := make(chan lambdaextensions.Log, 1)
			r := io.NopCloser(strings.NewReader(tt.response))
			err := lambdaextensions.DecodeLogs(r, logs)
			require.NoError(t, err)

			log := <-logs
			assert.Equal(t, tt.want.Time, log.Time)
			assert.Equal(t, tt.want.LogType, log.LogType)
			assert.JSONEq(t, string(tt.want.RawRecord), string(log.RawRecord))
			assert.Equal(t, tt.want.Record, log.Record)
		})
	}
}

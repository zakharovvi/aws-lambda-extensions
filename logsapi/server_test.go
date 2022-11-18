package logsapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/logsapi"
)

var (
	testIdentifier = "test-identifier"
	respRegister   = []byte(`
		{
			"functionName": "helloWorld",
			"functionVersion": "$LATEST",
			"lambdaAPIMock": "lambda_function.lambda_handler"
		}
	`)
	respError = []byte(`
		{
			"status": "OK"
		}
	`)
	respShutdown = []byte(`
		{
		  "eventType": "SHUTDOWN",
		  "shutdownReason": "spindown",
		  "deadlineMs": 9223372036854775807
		}
	`)
)

type testProcessor struct {
	initCalled     bool
	initErr        error
	receivedLogs   []logsapi.Log
	processErrors  []error
	shutdownErr    error
	shutdownCalled bool
}

func (proc *testProcessor) Init(ctx context.Context, client *extapi.Client) error {
	proc.initCalled = true

	return proc.initErr
}

func (proc *testProcessor) Process(ctx context.Context, msg logsapi.Log) error {
	proc.receivedLogs = append(proc.receivedLogs, msg)

	res := proc.processErrors[0]
	proc.processErrors = proc.processErrors[1:]

	return res
}

func (proc *testProcessor) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	proc.shutdownCalled = true

	return proc.shutdownErr
}

type lambdaAPIMock struct {
	t                   *testing.T
	wantDestinationURI  string
	logsRequests        [][]byte
	wantLogsResponses   []int
	logsSubscribeStatus int
	registerCalled      bool
	logsSubscribeCalled bool
	initErrorCalled     bool
	exitErrorCalled     bool
}

func (h *lambdaAPIMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/2020-01-01/extension/register":
		require.Falsef(h.t, h.registerCalled, "extension/register has already been called")
		h.registerCalled = true
		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		if _, err := w.Write(respRegister); err != nil {
			require.NoError(h.t, err, "extension/register")
		}
	case "/2020-01-01/extension/event/next":
		for _, logs := range h.logsRequests {
			req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, h.wantDestinationURI, bytes.NewReader(logs))
			require.NoError(h.t, err)

			resp, err := http.DefaultClient.Do(req)
			// request context can be cancelled for test cases with injected failures
			if err != nil {
				h.t.Log(err)

				break
			}

			require.Equal(h.t, h.wantLogsResponses[0], resp.StatusCode)
			h.wantLogsResponses = h.wantLogsResponses[1:]

			require.NoError(h.t, resp.Body.Close())
		}
		if _, err := w.Write(respShutdown); err != nil {
			require.NoError(h.t, err, "extension/event/next")
		}

	case "/2020-01-01/extension/init/error":
		require.Falsef(h.t, h.initErrorCalled, "extension/init/error has already been called")
		h.initErrorCalled = true
		if _, err := w.Write(respError); err != nil {
			require.NoError(h.t, err, "extension/init/error")
		}
	case "/2020-01-01/extension/exit/error":
		require.Falsef(h.t, h.exitErrorCalled, "extension/exit/error has already been called")
		h.exitErrorCalled = true
		if _, err := w.Write(respError); err != nil {
			require.NoError(h.t, err, "extension/exit/error")
		}
	case "/2020-08-15/logs":
		require.Falsef(h.t, h.logsSubscribeCalled, "logs has already been called")
		h.logsSubscribeCalled = true

		subscription := extapi.LogsSubscribeRequest{}
		require.NoError(h.t, json.NewDecoder(r.Body).Decode(&subscription))

		require.Equal(h.t, h.wantDestinationURI, subscription.Destination.URI)

		status := http.StatusOK
		if h.logsSubscribeStatus != 0 {
			status = h.logsSubscribeStatus
		}
		w.WriteHeader(status)
	default:
		require.Failf(h.t, "unknown url called: %s", r.URL.String())
		http.NotFound(w, r)
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name                    string
		apiMock                 *lambdaAPIMock
		proc                    *testProcessor
		destinationAddr         string
		wantReceivedLogs        []logsapi.Log
		wantRunErr              error
		wantLogsSubscribeCalled bool
		wantInitErrorCalled     bool
		wantExitErrorCalled     bool
	}{
		{
			"no logs",
			&lambdaAPIMock{},
			&testProcessor{},
			"localhost:10000",
			nil,
			nil,
			true,
			false,
			false,
		},
		{
			"server start failed",
			&lambdaAPIMock{},
			&testProcessor{},
			"127.0.0.1:1",
			nil,
			errors.New("Extension.Init failed: could not start event receiving HTTP server: listen tcp 127.0.0.1:1: bind: permission denied"),
			false,
			true,
			false,
		},
		{
			"client.LogsSubscribe failed",
			&lambdaAPIMock{
				logsSubscribeStatus: http.StatusInternalServerError,
			},
			&testProcessor{},
			"localhost:10000",
			nil,
			errors.New("Extension.Init failed: logs subscribe http call failed: http request failed with status 500 Internal Server Error and body: "),
			true,
			true,
			false,
		},
		{
			"multiple logs requests",
			&lambdaAPIMock{
				logsRequests: [][]byte{
					[]byte(`[{"type":"platform.end","time":"2022-01-01T00:00:00Z","record":{"requestId":"1.1"}},{"type":"platform.end","time":"2022-01-01T00:00:00Z","record":{"requestId":"1.2"}}]`),
					[]byte(`[{"type":"platform.end","time":"2022-01-01T00:00:00Z","record":{"requestId":"2.1"}},{"type":"platform.end","time":"2022-01-01T00:00:00Z","record":{"requestId":"2.2"}}]`),
				},
				wantLogsResponses: []int{http.StatusOK, http.StatusOK},
			},
			&testProcessor{
				processErrors: []error{nil, nil, nil, nil},
			},
			"localhost:10000",
			[]logsapi.Log{
				{
					logsapi.LogPlatformEnd,
					time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					json.RawMessage(`{"requestId":"1.1"}`),
					logsapi.RecordPlatformEnd{"1.1"},
				},
				{
					logsapi.LogPlatformEnd,
					time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					json.RawMessage(`{"requestId":"1.2"}`),
					logsapi.RecordPlatformEnd{"1.2"},
				},
				{
					logsapi.LogPlatformEnd,
					time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					json.RawMessage(`{"requestId":"2.1"}`),
					logsapi.RecordPlatformEnd{"2.1"},
				},
				{
					logsapi.LogPlatformEnd,
					time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					json.RawMessage(`{"requestId":"2.2"}`),
					logsapi.RecordPlatformEnd{"2.2"},
				},
			},
			nil,
			true,
			false,
			false,
		},
		{
			"invalid json",
			&lambdaAPIMock{
				logsRequests: [][]byte{
					[]byte(`[{"type":"platform.end","time":"2022-01-01T00:00:00Z","record":{"requestId":"1.1"}}, INVALID_JSON]`),
				},
				wantLogsResponses: []int{http.StatusInternalServerError},
			},
			&testProcessor{
				processErrors: []error{nil},
			},
			"localhost:10000",
			[]logsapi.Log{
				{
					logsapi.LogPlatformEnd,
					time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					json.RawMessage(`{"requestId":"1.1"}`),
					logsapi.RecordPlatformEnd{"1.1"},
				},
			},
			errors.New("extension loop failed: Extension.Err() signaled an error: decoding failed or interrupted: could not decode log message from json array: invalid character 'I' looking for beginning of value"),
			true,
			false,
			true,
		},
		{
			"Processor.Process failed",
			&lambdaAPIMock{
				logsRequests: [][]byte{
					[]byte(`[{"type":"platform.end","time":"2022-01-01T00:00:00Z","record":{"requestId":"1.1"}},{"type":"platform.end","time":"2022-01-01T00:00:00Z","record":{"requestId":"1.2"}}]`),
				},
				wantLogsResponses: []int{http.StatusOK},
			},
			&testProcessor{
				processErrors: []error{nil, errors.New("test_error")},
			},
			"localhost:10000",
			[]logsapi.Log{
				{
					logsapi.LogPlatformEnd,
					time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					json.RawMessage(`{"requestId":"1.1"}`),
					logsapi.RecordPlatformEnd{"1.1"},
				},
				{
					logsapi.LogPlatformEnd,
					time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					json.RawMessage(`{"requestId":"1.2"}`),
					logsapi.RecordPlatformEnd{"1.2"},
				},
			},
			errors.New("extension loop failed: Extension.Err() signaled an error: EventProcessor.Process failed: test_error"),
			true,
			false,
			true,
		},
		{
			"Processor.Shutdown failed",
			&lambdaAPIMock{
				logsRequests: [][]byte{
					[]byte(`[{"type":"platform.end","time":"2022-01-01T00:00:00Z","record":{"requestId":"1.1"}}]`),
				},
				wantLogsResponses: []int{http.StatusOK},
			},
			&testProcessor{
				processErrors: []error{nil},
				shutdownErr:   errors.New("shutdown_failed"),
			},
			"localhost:10000",
			[]logsapi.Log{
				{
					logsapi.LogPlatformEnd,
					time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					json.RawMessage(`{"requestId":"1.1"}`),
					logsapi.RecordPlatformEnd{"1.1"},
				},
			},
			errors.New("Extension.Shutdown failed: EventProcessor.Shutdown failed: shutdown_failed"),
			true,
			false,
			true,
		},
		{
			"Processor.Init failed",
			&lambdaAPIMock{},
			&testProcessor{
				initErr: errors.New("test error"),
			},
			"localhost:10000",
			nil,
			errors.New("Extension.Init failed: EventProcessor.Init failed: test error"),
			false,
			true,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.apiMock.t = t
			tt.apiMock.wantDestinationURI = "http://" + tt.destinationAddr
			server := httptest.NewServer(tt.apiMock)
			defer server.Close()
			t.Setenv("AWS_LAMBDA_RUNTIME_API", server.Listener.Addr().String())

			err := logsapi.Run(context.Background(), tt.proc, logsapi.WithDestinationAddr(tt.destinationAddr))
			if tt.wantRunErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantRunErr.Error())
			}
			require.True(t, tt.proc.initCalled)
			require.True(t, tt.proc.shutdownCalled)
			require.True(t, tt.apiMock.registerCalled)
			require.Equal(t, tt.wantLogsSubscribeCalled, tt.apiMock.logsSubscribeCalled)
			require.Equal(t, tt.wantInitErrorCalled, tt.apiMock.initErrorCalled)
			require.Equal(t, tt.wantExitErrorCalled, tt.apiMock.exitErrorCalled)
			require.Equal(t, tt.wantReceivedLogs, tt.proc.receivedLogs)
		})
	}
}

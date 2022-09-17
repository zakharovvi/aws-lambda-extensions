package extapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zakharovvi/lambda-extensions/extapi"
)

var (
	testIdentifier   = "test-identifier"
	testErrorType    = "extension.TestReason"
	testErrorMessage = "text description of the error"
	testErrorStatus  = "OK"

	respRegister = []byte(`
		{
			"functionName": "helloWorld",
			"functionVersion": "$LATEST",
			"handler": "lambda_function.lambda_handler"
		}
	`)

	respNextEvent []byte
	respInvoke    = []byte(`
		{
			"eventType": "INVOKE",
			"deadlineMs": 676051,
			"requestId": "3da1f2dc-3222-475e-9205-e2e6c6318895",
			"invokedFunctionArn": "arn:aws:lambda:us-east-1:123456789012:function:ExtensionTest",
			"tracing": {
				"type": "X-Amzn-Trace-Id",
				"value": "Root=1-5f35ae12-0c0fec141ab77a00bc047aa2;Parent=2be948a625588e32;Sampled=1"
			}
		}
	`)
	respShutdown = []byte(`
		{
		  "eventType": "SHUTDOWN",
		  "shutdownReason": "spindown",
		  "deadlineMs": 676051
		}
	`)
	respError = []byte(`
		{
			"status": "OK"
		}
	`)
)

func TestRegister(t *testing.T) {
	client, server, _, err := register(t)
	require.NoError(t, err)
	defer server.Close()

	assert.Equal(t, "helloWorld", client.RegisterResp.FunctionName)
	assert.Equal(t, "$LATEST", client.RegisterResp.FunctionVersion)
	assert.Equal(t, "lambda_function.lambda_handler", client.RegisterResp.Handler)
}

func TestNextEvent_Invoke(t *testing.T) {
	client, server, mux, err := register(t)
	require.NoError(t, err)
	defer server.Close()
	mux.HandleFunc("/2020-01-01/extension/event/next", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, testIdentifier, r.Header.Get("Lambda-Extension-Identifier"))

		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		if _, err := w.Write(respNextEvent); err != nil {
			t.Fatal(err)
		}
	})

	respNextEvent = respInvoke
	event, err := client.NextEvent(context.Background())
	require.NoError(t, err)

	assert.Equal(t, extapi.Invoke, event.EventType)
	assert.Equal(t, "3da1f2dc-3222-475e-9205-e2e6c6318895", event.RequestID)
	assert.Equal(t, "arn:aws:lambda:us-east-1:123456789012:function:ExtensionTest", event.InvokedFunctionArn)
	assert.Equal(t, "3da1f2dc-3222-475e-9205-e2e6c6318895", event.RequestID)
	assert.Equal(t, int64(676051), event.DeadlineMs)
	assert.Equal(t, "X-Amzn-Trace-Id", event.Tracing.Type)
	assert.Equal(t, "Root=1-5f35ae12-0c0fec141ab77a00bc047aa2;Parent=2be948a625588e32;Sampled=1", event.Tracing.Value)
}

func TestNextEvent_Shutdown(t *testing.T) {
	client, server, mux, err := register(t)
	require.NoError(t, err)
	defer server.Close()
	mux.HandleFunc("/2020-01-01/extension/event/next", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, testIdentifier, r.Header.Get("Lambda-Extension-Identifier"))

		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		if _, err := w.Write(respNextEvent); err != nil {
			t.Fatal(err)
		}
	})

	respNextEvent = respShutdown
	event, err := client.NextEvent(context.Background())
	require.NoError(t, err)
	assert.Equal(t, extapi.Shutdown, event.EventType)
	assert.Equal(t, extapi.Spindown, event.ShutdownReason)
	assert.Equal(t, int64(676051), event.DeadlineMs)
}

func TestInitError(t *testing.T) {
	client, server, mux, err := register(t)
	require.NoError(t, err)
	defer server.Close()
	mux.HandleFunc("/2020-01-01/extension/init/error", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, testIdentifier, r.Header.Get("Lambda-Extension-Identifier"))
		assert.Equal(t, testErrorType, r.Header.Get("Lambda-Extension-Function-Error-Type"))

		req, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		// body can be empty
		if len(req) != 0 {
			assert.JSONEq(t, `{"errorMessage": "text description of the error", "errorType": "extension.TestReason", "stackTrace": null}`, string(req))
		}

		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		if _, err := w.Write(respError); err != nil {
			t.Fatal(err)
		}
	})

	tests := []struct {
		name     string
		errorReq *extapi.ErrorRequest
	}{
		{
			name:     "nil request",
			errorReq: nil,
		},
		{
			name: "with request",
			errorReq: &extapi.ErrorRequest{
				ErrorMessage: testErrorMessage,
				ErrorType:    testErrorType,
				StackTrace:   nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := client.InitError(context.Background(), testErrorType, tt.errorReq)
			require.NoError(t, err)
			assert.Equal(t, testErrorStatus, status.Status)
		})
	}
}

func TestExitError(t *testing.T) {
	client, server, mux, err := register(t)
	require.NoError(t, err)
	defer server.Close()
	mux.HandleFunc("/2020-01-01/extension/exit/error", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, testIdentifier, r.Header.Get("Lambda-Extension-Identifier"))
		assert.Equal(t, testErrorType, r.Header.Get("Lambda-Extension-Function-Error-Type"))

		req, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		// body can be empty
		if len(req) != 0 {
			assert.JSONEq(t, `{"errorMessage": "text description of the error", "errorType": "extension.TestReason", "stackTrace": null}`, string(req))
		}

		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		if _, err := w.Write(respError); err != nil {
			t.Fatal(err)
		}
	})

	tests := []struct {
		name     string
		errorReq *extapi.ErrorRequest
	}{
		{
			name:     "nil request",
			errorReq: nil,
		},
		{
			name: "with request",
			errorReq: &extapi.ErrorRequest{
				ErrorMessage: testErrorMessage,
				ErrorType:    testErrorType,
				StackTrace:   nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := client.ExitError(context.Background(), testErrorType, tt.errorReq)
			require.NoError(t, err)
			assert.Equal(t, testErrorStatus, status.Status)
		})
	}
}

func register(t *testing.T) (*extapi.Client, *httptest.Server, *http.ServeMux, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/2020-01-01/extension/register", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, filepath.Base(os.Args[0]), r.Header.Get("Lambda-Extension-Name"))

		req, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		assert.JSONEq(t, `{"events":["INVOKE","SHUTDOWN"]}`, string(req))

		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		if _, err := w.Write(respRegister); err != nil {
			t.Fatal(err)
		}
	})
	server := httptest.NewServer(mux)

	if err := os.Setenv("AWS_LAMBDA_RUNTIME_API", server.Listener.Addr().String()); err != nil {
		t.Fatal(err)
	}
	client, err := extapi.Register(context.Background())
	return client, server, mux, err
}

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

		subscribeReq := &extapi.LogsSubscribeRequest{}
		assert.NoError(t, json.Unmarshal(req, subscribeReq))
		assert.Equal(t, logReceiverURL, subscribeReq.Destination.URI)
		assert.Equal(
			t,
			[]extapi.LogSubscriptionType{extapi.Platform, extapi.Function, extapi.Extension},
			subscribeReq.LogTypes,
		)

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Fatal(err)
		}
	})

	subscribeReq := extapi.NewLogsSubscribeRequest(logReceiverURL, nil)
	err = client.LogsSubscribe(context.Background(), subscribeReq)
	assert.NoError(t, err)
}

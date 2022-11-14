package extapi_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
)

var (
	testIdentifier  = "test-identifier"
	testErrorType   = "extension.TestReason"
	testErrorStatus = "OK"
	errTest         = errors.New("text description of the error")

	respRegister = []byte(`
		{
			"functionName": "helloWorld",
			"functionVersion": "$LATEST",
			"handler": "lambda_function.lambda_handler",
			"accountId": "123456789012"
		}
	`)

	respNextEvent []byte
	respInvoke    = []byte(`
		{
			"eventType": "INVOKE",
			"deadlineMs": 9223372036854775807,
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
		  "deadlineMs": 9223372036854775807
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

	require.Equal(t, "helloWorld", client.FunctionName())
	require.Equal(t, "$LATEST", client.FunctionVersion())
	require.Equal(t, "lambda_function.lambda_handler", client.Handler())
	require.Equal(t, "123456789012", client.AccountID())
	require.Equal(t, testIdentifier, client.ExtensionID())
}

func TestLambdaAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/2020-01-01/extension/register", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"errorType": "ValidationError", "errorMessage": "URI port is not provided; types should not be empty"}`)); err != nil {
			t.Fatal(err)
		}
	})
	server := httptest.NewServer(mux)

	t.Setenv("AWS_LAMBDA_RUNTIME_API", server.Listener.Addr().String())
	_, err := extapi.Register(context.Background())
	require.ErrorIs(t, err, extapi.LambdaAPIError{
		Type:           "ValidationError",
		Message:        "URI port is not provided; types should not be empty",
		HTTPStatusCode: http.StatusBadRequest,
	})
}

func TestNextEvent_Invoke(t *testing.T) {
	client, server, mux, err := register(t)
	require.NoError(t, err)
	defer server.Close()
	mux.HandleFunc("/2020-01-01/extension/event/next", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, testIdentifier, r.Header.Get("Lambda-Extension-Identifier"))

		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		if _, err := w.Write(respNextEvent); err != nil {
			t.Fatal(err)
		}
	})

	respNextEvent = respInvoke
	event, err := client.NextEvent(context.Background())
	require.NoError(t, err)

	require.Equal(t, extapi.Invoke, event.EventType)
	require.Equal(t, "3da1f2dc-3222-475e-9205-e2e6c6318895", event.RequestID)
	require.Equal(t, "arn:aws:lambda:us-east-1:123456789012:function:ExtensionTest", event.InvokedFunctionArn)
	require.Equal(t, "3da1f2dc-3222-475e-9205-e2e6c6318895", event.RequestID)
	require.Equal(t, int64(9223372036854775807), event.DeadlineMs)
	require.Equal(t, "X-Amzn-Trace-Id", event.Tracing.Type)
	require.Equal(t, "Root=1-5f35ae12-0c0fec141ab77a00bc047aa2;Parent=2be948a625588e32;Sampled=1", event.Tracing.Value)
}

func TestNextEvent_Shutdown(t *testing.T) {
	client, server, mux, err := register(t)
	require.NoError(t, err)
	defer server.Close()
	mux.HandleFunc("/2020-01-01/extension/event/next", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, testIdentifier, r.Header.Get("Lambda-Extension-Identifier"))

		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		if _, err := w.Write(respNextEvent); err != nil {
			t.Fatal(err)
		}
	})

	respNextEvent = respShutdown
	event, err := client.NextEvent(context.Background())
	require.NoError(t, err)
	require.Equal(t, extapi.Shutdown, event.EventType)
	require.Equal(t, extapi.Spindown, event.ShutdownReason)
	require.Equal(t, int64(9223372036854775807), event.DeadlineMs)
}

func TestInitError(t *testing.T) {
	client, server, mux, err := register(t)
	require.NoError(t, err)
	defer server.Close()
	mux.HandleFunc("/2020-01-01/extension/init/error", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, testIdentifier, r.Header.Get("Lambda-Extension-Identifier"))
		require.Equal(t, testErrorType, r.Header.Get("Lambda-Extension-Function-Error-Type"))

		req, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		require.Equal(t, errTest.Error(), string(req))

		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		w.WriteHeader(http.StatusAccepted)
		if _, err := w.Write(respError); err != nil {
			t.Fatal(err)
		}
	})

	tests := []struct {
		name      string
		errorType string
		err       error
	}{
		{
			name:      "with request",
			errorType: testErrorType,
			err:       errTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := client.InitError(context.Background(), tt.errorType, tt.err)
			require.NoError(t, err)
			require.Equal(t, testErrorStatus, status.Status)
		})
	}
}

func TestExitError(t *testing.T) {
	client, server, mux, err := register(t)
	require.NoError(t, err)
	defer server.Close()
	mux.HandleFunc("/2020-01-01/extension/exit/error", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, testIdentifier, r.Header.Get("Lambda-Extension-Identifier"))
		require.Equal(t, testErrorType, r.Header.Get("Lambda-Extension-Function-Error-Type"))

		req, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		require.Equal(t, errTest.Error(), string(req))

		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		w.WriteHeader(http.StatusAccepted)
		if _, err := w.Write(respError); err != nil {
			t.Fatal(err)
		}
	})

	tests := []struct {
		name      string
		errorType string
		err       error
	}{
		{
			name:      "with request",
			errorType: testErrorType,
			err:       errTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := client.ExitError(context.Background(), tt.errorType, tt.err)
			require.NoError(t, err)
			require.Equal(t, testErrorStatus, status.Status)
		})
	}
}

func register(t *testing.T) (*extapi.Client, *httptest.Server, *http.ServeMux, error) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/2020-01-01/extension/register", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, filepath.Base(os.Args[0]), r.Header.Get("Lambda-Extension-Name"))

		req, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		require.JSONEq(t, `{"events":["INVOKE","SHUTDOWN"]}`, string(req))

		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		if _, err := w.Write(respRegister); err != nil {
			t.Fatal(err)
		}
	})
	server := httptest.NewServer(mux)

	t.Setenv("AWS_LAMBDA_RUNTIME_API", server.Listener.Addr().String())
	client, err := extapi.Register(context.Background())

	return client, server, mux, err
}

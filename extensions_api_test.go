package lambdaextensions_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zakharovvi/lambdaextensions"
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
	client, server, err := register(t)
	require.NoError(t, err)
	defer server.Close()

	assert.Equal(t, "helloWorld", client.RegisterResp.FunctionName)
	assert.Equal(t, "$LATEST", client.RegisterResp.FunctionVersion)
	assert.Equal(t, "lambda_function.lambda_handler", client.RegisterResp.Handler)
}

func TestNextEvent_Invoke(t *testing.T) {
	client, server, err := register(t)
	require.NoError(t, err)
	defer server.Close()

	respNextEvent = respInvoke
	event, err := client.NextEvent(context.Background())
	require.NoError(t, err)

	assert.Equal(t, lambdaextensions.Invoke, event.EventType)
	assert.Equal(t, "3da1f2dc-3222-475e-9205-e2e6c6318895", event.RequestID)
	assert.Equal(t, "arn:aws:lambda:us-east-1:123456789012:function:ExtensionTest", event.InvokedFunctionArn)
	assert.Equal(t, "3da1f2dc-3222-475e-9205-e2e6c6318895", event.RequestID)
	assert.Equal(t, int64(676051), event.DeadlineMs)
	assert.Equal(t, "X-Amzn-Trace-Id", event.Tracing.Type)
	assert.Equal(t, "Root=1-5f35ae12-0c0fec141ab77a00bc047aa2;Parent=2be948a625588e32;Sampled=1", event.Tracing.Value)
}

func TestNextEvent_Shutdown(t *testing.T) {
	client, server, err := register(t)
	require.NoError(t, err)
	defer server.Close()

	respNextEvent = respShutdown
	event, err := client.NextEvent(context.Background())
	require.NoError(t, err)
	assert.Equal(t, lambdaextensions.Shutdown, event.EventType)
	assert.Equal(t, lambdaextensions.Spindown, event.ShutdownReason)
	assert.Equal(t, int64(676051), event.DeadlineMs)
}

func TestInitError(t *testing.T) {
	client, server, err := register(t)
	require.NoError(t, err)
	defer server.Close()

	tests := []struct {
		name     string
		errorReq *lambdaextensions.ErrorRequest
	}{
		{
			name:     "nil request",
			errorReq: nil,
		},
		{
			name: "with request",
			errorReq: &lambdaextensions.ErrorRequest{
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
	client, server, err := register(t)
	require.NoError(t, err)
	defer server.Close()

	tests := []struct {
		name     string
		errorReq *lambdaextensions.ErrorRequest
	}{
		{
			name:     "nil request",
			errorReq: nil,
		},
		{
			name: "with request",
			errorReq: &lambdaextensions.ErrorRequest{
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

func register(t *testing.T) (*lambdaextensions.Client, *httptest.Server, error) {
	server := startServer(t)

	if err := os.Setenv("AWS_LAMBDA_RUNTIME_API", server.Listener.Addr().String()); err != nil {
		t.Fatal(err)
	}

	client, err := lambdaextensions.Register(context.Background())
	return client, server, err
}

func startServer(t *testing.T) *httptest.Server {
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
	mux.HandleFunc("/2020-01-01/extension/event/next", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, testIdentifier, r.Header.Get("Lambda-Extension-Identifier"))

		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		if _, err := w.Write(respNextEvent); err != nil {
			t.Fatal(err)
		}
	})
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

	return httptest.NewServer(mux)
}

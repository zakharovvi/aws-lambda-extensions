package extapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
)

type testExtension struct {
	t                     *testing.T
	events                []*extapi.NextEventResponse
	handleInvokeEventErrs []error
	initErr               error
	shutdownErr           error
	initCalled            bool
	shutdownCalled        bool
}

func (ext *testExtension) Init(ctx context.Context, client *extapi.Client) error {
	require.Falsef(ext.t, ext.initCalled, "Init has already been called")
	ext.initCalled = true

	return ext.initErr
}

func (ext *testExtension) HandleInvokeEvent(ctx context.Context, event *extapi.NextEventResponse) error {
	ext.events = append(ext.events, event)

	res := ext.handleInvokeEventErrs[0]
	ext.handleInvokeEventErrs = ext.handleInvokeEventErrs[1:]

	return res
}

func (ext *testExtension) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	require.Falsef(ext.t, ext.shutdownCalled, "Shutdown has already been called")
	ext.shutdownCalled = true

	return ext.shutdownErr
}

func (ext *testExtension) Err() <-chan error {
	return nil
}

type lambdaAPIMock struct {
	t               *testing.T
	events          [][]byte
	registerCalled  bool
	initErrorCalled bool
	exitErrorCalled bool
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
		if len(h.events) == 0 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			e := h.events[0]
			h.events = h.events[1:]
			if _, err := w.Write(e); err != nil {
				require.NoError(h.t, err, "extension/event/next")
			}
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
	default:
		require.Failf(h.t, "unknown url called: %s", r.URL.String())
		http.NotFound(w, r)
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name                string
		handler             *lambdaAPIMock
		ext                 *testExtension
		wantRunErr          error
		wantInitErrorCalled bool
		wantExitErrorCalled bool
	}{
		{
			"simple",
			&lambdaAPIMock{
				events: [][]byte{respInvoke, respInvoke, respShutdown},
			},
			&testExtension{
				handleInvokeEventErrs: []error{nil, nil},
			},
			nil,
			false,
			false,
		},
		{
			"Extension.Init failed",
			&lambdaAPIMock{},
			&testExtension{
				initErr: errors.New("internal error"),
			},
			errors.New("Extension.Init failed: internal error"),
			true,
			false,
		},
		{
			"Client.NextEvent failed",
			&lambdaAPIMock{
				events: [][]byte{{}},
			},
			&testExtension{},
			errors.New("extension loop failed: Client.NextEvent failed: event/next call failed: could not json decode http response : unexpected end of JSON input"),
			false,
			true,
		},
		{
			"Extension.HandleInvokeEvent failed",
			&lambdaAPIMock{
				events: [][]byte{respInvoke},
			},
			&testExtension{
				handleInvokeEventErrs: []error{errors.New("internal error")},
			},
			errors.New("extension loop failed: Extension.HandleInvokeEvent failed: internal error"),
			false,
			true,
		},
		{
			"Extension.Shutdown failed",
			&lambdaAPIMock{
				events: [][]byte{respShutdown},
			},
			&testExtension{
				shutdownErr: errors.New("Extension.Shutdown failed"),
			},
			errors.New("Extension.Shutdown failed: Extension.Shutdown failed"),
			false,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.handler.t = t
			tt.ext.t = t

			server := httptest.NewServer(tt.handler)
			defer server.Close()
			t.Setenv("AWS_LAMBDA_RUNTIME_API", server.Listener.Addr().String())

			err := extapi.Run(context.Background(), tt.ext)

			if tt.wantRunErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantRunErr.Error())
			}
			require.True(t, tt.ext.initCalled)
			require.True(t, tt.ext.shutdownCalled)
			require.True(t, tt.handler.registerCalled)
			require.Equal(t, tt.wantInitErrorCalled, tt.handler.initErrorCalled)
			require.Equal(t, tt.wantExitErrorCalled, tt.handler.exitErrorCalled)
		})
	}
}

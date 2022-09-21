package extapi_test

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zakharovvi/lambda-extensions/extapi"
)

type testExtension struct {
	events                []*extapi.NextEventResponse
	handleInvokeEventErrs []error
	initErr               error
	shutdownErr           error
	initCalled            bool
	shutdownCalled        bool
}

func (te *testExtension) Init(ctx context.Context, client *extapi.Client) error {
	if te.initCalled {
		panic("Init has already been called")
	}
	te.initCalled = true

	return te.initErr
}

func (te *testExtension) HandleInvokeEvent(ctx context.Context, event *extapi.NextEventResponse) error {
	te.events = append(te.events, event)

	res := te.handleInvokeEventErrs[0]
	te.handleInvokeEventErrs = te.handleInvokeEventErrs[1:]

	return res
}

func (te *testExtension) Shutdown(ctx context.Context, reason extapi.ShutdownReason) error {
	if te.shutdownCalled {
		panic("Shutdown has already been called")
	}
	te.shutdownCalled = true

	return te.shutdownErr
}

type handler struct {
	events          [][]byte
	registerCalled  bool
	initErrorCalled bool
	exitErrorCalled bool
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/2020-01-01/extension/register":
		if h.registerCalled {
			panic("register has already been called")
		}
		h.registerCalled = true
		w.Header().Set("Lambda-Extension-Identifier", testIdentifier)
		if _, err := w.Write(respRegister); err != nil {
			log.Panic(err)
		}
	case "/2020-01-01/extension/event/next":
		if len(h.events) == 0 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			e := h.events[0]
			h.events = h.events[1:]
			if _, err := w.Write(e); err != nil {
				log.Panic(err)
			}
		}
	case "/2020-01-01/extension/init/error":
		if h.initErrorCalled {
			panic("/init/error has already been called")
		}
		h.initErrorCalled = true
		if _, err := w.Write(respError); err != nil {
			log.Panic(err)
		}
	case "/2020-01-01/extension/exit/error":
		if h.exitErrorCalled {
			panic("exit/error has already been called")
		}
		h.exitErrorCalled = true
		if _, err := w.Write(respError); err != nil {
			log.Panic(err)
		}
	default:
		http.NotFound(w, r)
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name                string
		handler             *handler
		ext                 *testExtension
		wantRunErr          error
		wantInitErrorCalled bool
		wantExitErrorCalled bool
	}{
		{
			"simple",
			&handler{
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
			&handler{},
			&testExtension{
				initErr: errors.New("internal error"),
			},
			errors.New("Extension.Init failed: internal error"),
			true,
			false,
		},
		{
			"Client.NextEvent failed",
			&handler{
				events: [][]byte{{}},
			},
			&testExtension{},
			errors.New("extension loop failed: event/next call failed: could not json decode http response : unexpected end of JSON input"),
			false,
			true,
		},
		{
			"Extension.HandleInvokeEvent failed",
			&handler{
				events: [][]byte{respInvoke},
			},
			&testExtension{
				handleInvokeEventErrs: []error{errors.New("internal error")},
			},
			errors.New("extension loop failed: internal error"),
			false,
			true,
		},
		{
			"Extension.Shutdown failed",
			&handler{
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

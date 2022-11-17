package extapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
)

const (
	logReceiverURL = "http://sandbox.localdomain:8080/logs"
)

func TestLogsSubscribe(t *testing.T) {
	client, server, mux, err := register(t)
	require.NoError(t, err)
	defer server.Close()
	mux.HandleFunc("/2020-08-15/logs", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, testExtensionID, r.Header.Get("Lambda-Extension-Identifier"))

		req, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		subscribeReq := &extapi.LogsSubscribeRequest{}
		require.NoError(t, json.Unmarshal(req, subscribeReq))

		want := &extapi.LogsSubscribeRequest{
			SchemaVersion: extapi.LogsSchemaVersion20210318,
			LogTypes:      []extapi.LogSubscriptionType{extapi.LogSubscriptionTypePlatform, extapi.LogSubscriptionTypeFunction},
			BufferingCfg:  nil,
			Destination: &extapi.LogsDestination{
				Protocol:   "HTTP",
				URI:        logReceiverURL,
				HTTPMethod: "",
				Encoding:   "",
			},
		}
		require.Equal(t, want, subscribeReq)

		_, err = w.Write([]byte("OK"))
		require.NoError(t, err)
	})

	subscribeReq := extapi.NewLogsSubscribeRequest(logReceiverURL, nil, nil)
	err = client.LogsSubscribe(context.Background(), subscribeReq)
	require.NoError(t, err)
}

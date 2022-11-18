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
	telemetryReceiverURL = "http://sandbox.localdomain:8080/telemetry"
)

func TestTelemetrySubscribe(t *testing.T) {
	client, server, mux, err := register(t)
	require.NoError(t, err)
	defer server.Close()
	mux.HandleFunc("/2022-07-01/telemetry", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, testExtensionID, r.Header.Get("Lambda-Extension-Identifier"))

		req, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		subscribeReq := &extapi.TelemetrySubscribeRequest{}
		require.NoError(t, json.Unmarshal(req, subscribeReq))

		want := &extapi.TelemetrySubscribeRequest{
			SchemaVersion: extapi.TelemetrySchemaVersion20220701,
			Types:         []extapi.TelemetrySubscriptionType{extapi.TelemetrySubscriptionTypePlatform, extapi.TelemetrySubscriptionTypeFunction},
			BufferingCfg:  nil,
			Destination: &extapi.TelemetryDestination{
				Protocol: "HTTP",
				URI:      telemetryReceiverURL,
			},
		}
		require.Equal(t, want, subscribeReq)

		_, err = w.Write([]byte("OK"))
		require.NoError(t, err)
	})

	subscribeReq := extapi.NewTelemetrySubscribeRequest(telemetryReceiverURL, nil, nil)
	err = client.TelemetrySubscribe(context.Background(), subscribeReq)
	require.NoError(t, err)
}

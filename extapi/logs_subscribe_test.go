package extapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
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

		subscribeReq := &extapi.LogsSubscribeRequest{}
		assert.NoError(t, json.Unmarshal(req, subscribeReq))
		assert.Equal(t, logReceiverURL, subscribeReq.Destination.URI)
		assert.Equal(
			t,
			[]extapi.LogSubscriptionType{extapi.LogSubscriptionTypePlatform, extapi.LogSubscriptionTypeFunction, extapi.LogSubscriptionTypeExtension},
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

package extapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// TelemetrySubscriptionType represents the type of telemetry events in Lambda.
type TelemetrySubscriptionType string

const (
	// TelemetrySubscriptionTypePlatform is logs, metrics, and traces, which describe events and errors
	// related to the execution environment runtime lifecycle, extension lifecycle, and function invocations.
	TelemetrySubscriptionTypePlatform TelemetrySubscriptionType = "platform"
	// TelemetrySubscriptionTypeFunction is custom logs that the Lambda function code generates.
	TelemetrySubscriptionTypeFunction TelemetrySubscriptionType = "function"
	// TelemetrySubscriptionTypeExtension is custom logs that the Lambda extension code generates.
	TelemetrySubscriptionTypeExtension TelemetrySubscriptionType = "extension"
)

// TelemetryBufferingCfg is the configuration set for receiving events from Telemetry API. Whichever of the conditions below is met first, the events will be sent.
type TelemetryBufferingCfg struct {
	// MaxItems is the maximum number of events to be buffered in memory. (default: 10000, minimum: 1000, maximum: 10000)
	MaxItems uint32 `json:"maxItems"`
	// MaxBytes is the maximum size in bytes of data to be buffered in memory. (default: 262144, minimum: 262144, maximum: 1048576)
	MaxBytes uint32 `json:"maxBytes"`
	// TimeoutMS is the maximum time (in milliseconds) for a batch to be buffered. (default: 1000, minimum: 100, maximum: 30000)
	TimeoutMS uint32 `json:"timeoutMs"`
}

// TelemetryDestination is the configuration settings that define the telemetry event destination and the protocol for event delivery.
type TelemetryDestination struct {
	Protocol string `json:"protocol"`
	URI      string `json:"URI"`
}

type TelemetrySchemaVersion string

const (
	TelemetrySchemaVersion20220701 TelemetrySchemaVersion = "2022-07-01"
)

// TelemetrySubscribeRequest is the request body that is sent to Telemetry API on subscribe.
type TelemetrySubscribeRequest struct {
	SchemaVersion TelemetrySchemaVersion      `json:"schemaVersion,omitempty"`
	Types         []TelemetrySubscriptionType `json:"types"`
	BufferingCfg  *TelemetryBufferingCfg      `json:"buffering,omitempty"`
	Destination   *TelemetryDestination       `json:"destination"`
}

// NewTelemetrySubscribeRequest creates TelemetrySubscribeRequest with sensible defaults.
func NewTelemetrySubscribeRequest(url string, types []TelemetrySubscriptionType, bufferingCfg *TelemetryBufferingCfg) *TelemetrySubscribeRequest {
	if len(types) == 0 {
		// do not subscribe to TelemetrySubscriptionTypeExtension by default to avoid recursion
		types = append(types, TelemetrySubscriptionTypePlatform, TelemetrySubscriptionTypeFunction)
	}

	return &TelemetrySubscribeRequest{
		SchemaVersion: TelemetrySchemaVersion20220701,
		Types:         types,
		BufferingCfg:  bufferingCfg,
		Destination: &TelemetryDestination{
			Protocol: "HTTP",
			URI:      url,
		},
	}
}

// TelemetrySubscribe subscribes to a telemetry stream
// Lambda streams the telemetry to the extension, and the extension can then process, filter, and send the logs to any preferred destination.
// Subscription should occur during the extension initialization phase.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-api-reference.html
func (c *Client) TelemetrySubscribe(ctx context.Context, subscribeReq *TelemetrySubscribeRequest) error {
	body, err := json.Marshal(subscribeReq)
	if err != nil {
		err = fmt.Errorf("could not json encode telemetry subscribe request: %w", err)
		c.log.Error(err, "")

		return err
	}
	url := fmt.Sprintf("http://%s/2022-07-01/telemetry", c.awsLambdaRuntimeAPI)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		err = fmt.Errorf("could not telemetry subscribe http request: %w", err)
		c.log.Error(err, "")

		return err
	}

	if _, err := c.doRequest(req, http.StatusOK, nil); err != nil {
		err = fmt.Errorf("telemetry subscribe http call failed: %w", err)
		c.log.Error(err, "")

		return err
	}

	return nil
}

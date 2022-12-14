package extapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// LogSubscriptionType represents the type of logs in Lambda.
type LogSubscriptionType string

const (
	// LogSubscriptionTypePlatform is to receive logs emitted by the platform.
	LogSubscriptionTypePlatform LogSubscriptionType = "platform"
	// LogSubscriptionTypeFunction is to receive logs emitted by the function.
	LogSubscriptionTypeFunction LogSubscriptionType = "function"
	// LogSubscriptionTypeExtension is to receive logs emitted by the extension.
	LogSubscriptionTypeExtension LogSubscriptionType = "extension"
)

// LogsBufferingCfg is the configuration set for receiving logs from Logs API. Whichever of the conditions below is met first, the logs will be sent.
type LogsBufferingCfg struct {
	// MaxItems is the maximum number of events to be buffered in memory. (default: 10000, minimum: 1000, maximum: 10000)
	MaxItems uint32 `json:"maxItems"`
	// MaxBytes is the maximum size in bytes of the logs to be buffered in memory. (default: 262144, minimum: 262144, maximum: 1048576)
	MaxBytes uint32 `json:"maxBytes"`
	// TimeoutMS is the maximum time (in milliseconds) for a batch to be buffered. (default: 1000, minimum: 100, maximum: 30000)
	TimeoutMS uint32 `json:"timeoutMs"`
}

// LogsHTTPMethod represents the HTTP method used to receive logs from Logs API.
type LogsHTTPMethod string

const (
	// HTTPPost is to receive logs through POST.
	HTTPPost LogsHTTPMethod = "POST"
	// HTTPPut is to receive logs through PUT.
	HTTPPut LogsHTTPMethod = "PUT"
)

// LogsHTTPProtocol is used to specify the protocol when subscribing to Logs API for HTTP.
type LogsHTTPProtocol string

const (
	HTTPProto LogsHTTPProtocol = "HTTP"
)

// LogsHTTPEncoding denotes what the content is encoded in.
type LogsHTTPEncoding string

const (
	JSON LogsHTTPEncoding = "JSON"
)

// LogsDestination is the configuration for listeners who would like to receive logs with HTTP.
type LogsDestination struct {
	Protocol   LogsHTTPProtocol `json:"protocol"`
	URI        string           `json:"URI"`
	HTTPMethod LogsHTTPMethod   `json:"method,omitempty"`
	Encoding   LogsHTTPEncoding `json:"encoding,omitempty"`
}

type LogsSchemaVersion string

const (
	LogsSchemaVersion20210318 LogsSchemaVersion = "2021-03-18"
)

// LogsSubscribeRequest is the request body that is sent to Logs API on subscribe.
type LogsSubscribeRequest struct {
	SchemaVersion LogsSchemaVersion     `json:"schemaVersion,omitempty"`
	LogTypes      []LogSubscriptionType `json:"types"`
	BufferingCfg  *LogsBufferingCfg     `json:"buffering,omitempty"`
	Destination   *LogsDestination      `json:"destination"`
}

// NewLogsSubscribeRequest creates LogsSubscribeRequest with sensible defaults.
//
// Deprecated: The Lambda Telemetry API supersedes the Lambda Logs API. Use NewTelemetrySubscribeRequest instead.
func NewLogsSubscribeRequest(url string, logTypes []LogSubscriptionType, bufferingCfg *LogsBufferingCfg) *LogsSubscribeRequest {
	if len(logTypes) == 0 {
		// do not subscribe to LogSubscriptionTypeExtension by default to avoid recursion
		logTypes = append(logTypes, LogSubscriptionTypePlatform, LogSubscriptionTypeFunction)
	}

	return &LogsSubscribeRequest{
		SchemaVersion: LogsSchemaVersion20210318,
		LogTypes:      logTypes,
		BufferingCfg:  bufferingCfg,
		Destination: &LogsDestination{
			Protocol: HTTPProto,
			URI:      url,
		},
	}
}

// LogsSubscribe subscribes to a log stream.
// Lambda streams the logs to the extension, and the extension can then process, filter, and send the logs to any preferred destination.
// Subscription should occur during the extension initialization phase.
//
// Deprecated: The Lambda Telemetry API supersedes the Lambda Logs API. Use TelemetrySubscribe instead.
//
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-api-reference.html
func (c *Client) LogsSubscribe(ctx context.Context, subscribeReq *LogsSubscribeRequest) error {
	body, err := json.Marshal(subscribeReq)
	if err != nil {
		err = fmt.Errorf("could not json encode logs subscribe request: %w", err)
		c.log.Error(err, "")

		return err
	}
	url := fmt.Sprintf("http://%s/2020-08-15/logs", c.awsLambdaRuntimeAPI)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		err = fmt.Errorf("could not logs subscribe http request: %w", err)
		c.log.Error(err, "")

		return err
	}

	if _, err := c.doRequest(req, http.StatusOK, nil); err != nil {
		err = fmt.Errorf("logs subscribe http call failed: %w", err)
		c.log.Error(err, "")

		return err
	}

	return nil
}

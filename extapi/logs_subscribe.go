package extapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// LogSubscriptionType represents the type of logs in Lambda
type LogSubscriptionType string

const (
	// LogSubscriptionTypePlatform is to receive logs emitted by the platform
	LogSubscriptionTypePlatform LogSubscriptionType = "platform"
	// LogSubscriptionTypeFunction is to receive logs emitted by the function
	LogSubscriptionTypeFunction LogSubscriptionType = "function"
	// LogSubscriptionTypeExtension is to receive logs emitted by the extension
	LogSubscriptionTypeExtension LogSubscriptionType = "extension"
)

// LogsBufferingCfg is the configuration set for receiving logs from Logs API. Whichever of the conditions below is met first, the logs will be sent
type LogsBufferingCfg struct {
	// MaxItems is the maximum number of events to be buffered in memory. (default: 10000, minimum: 1000, maximum: 10000)
	MaxItems uint32 `json:"maxItems"`
	// MaxBytes is the maximum size in bytes of the logs to be buffered in memory. (default: 262144, minimum: 262144, maximum: 1048576)
	MaxBytes uint32 `json:"maxBytes"`
	// TimeoutMS is the maximum time (in milliseconds) for a batch to be buffered. (default: 1000, minimum: 100, maximum: 30000)
	TimeoutMS uint32 `json:"timeoutMs"`
}

// LogsHTTPMethod represents the HTTP method used to receive logs from Logs API
type LogsHTTPMethod string

const (
	//HttpPost is to receive logs through POST.
	HttpPost LogsHTTPMethod = "POST"
	// HttpPut is to receive logs through PUT.
	HttpPut LogsHTTPMethod = "PUT"
)

// LogsHttpProtocol is used to specify the protocol when subscribing to Logs API for HTTP
type LogsHttpProtocol string

const (
	HttpProto LogsHttpProtocol = "HTTP"
)

// LogsHttpEncoding denotes what the content is encoded in
type LogsHttpEncoding string

const (
	JSON LogsHttpEncoding = "JSON"
)

// LogsDestination is the configuration for listeners who would like to receive logs with HTTP
type LogsDestination struct {
	Protocol   LogsHttpProtocol `json:"protocol"`
	URI        string           `json:"URI"`
	HttpMethod LogsHTTPMethod   `json:"method,omitempty"`
	Encoding   LogsHttpEncoding `json:"encoding,omitempty"`
}

type LogsSchemaVersion string

const (
	LogsSchemaVersion20210318 LogsSchemaVersion = "2021-03-18"
)

// LogsSubscribeRequest is the request body that is sent to Logs API on subscribe
type LogsSubscribeRequest struct {
	SchemaVersion LogsSchemaVersion     `json:"schemaVersion,omitempty"`
	LogTypes      []LogSubscriptionType `json:"types"`
	BufferingCfg  *LogsBufferingCfg     `json:"buffering,omitempty"`
	Destination   *LogsDestination      `json:"destination"`
}

func NewLogsSubscribeRequest(url string, logTypes []LogSubscriptionType) *LogsSubscribeRequest {
	if len(logTypes) == 0 {
		logTypes = append(logTypes, LogSubscriptionTypePlatform, LogSubscriptionTypeFunction, LogSubscriptionTypeExtension)
	}
	return &LogsSubscribeRequest{
		LogTypes: logTypes,
		Destination: &LogsDestination{
			Protocol: HttpProto,
			URI:      url,
		},
	}
}

func (c *Client) LogsSubscribe(ctx context.Context, subscribeReq *LogsSubscribeRequest) error {
	body, err := json.Marshal(subscribeReq)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://%s/2020-08-15/logs", c.runtimeAPI)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set(idHeader, c.extensionID)
	req.Header.Set("Content-Type", "application/json")

	if _, err := c.doRequest(req, http.StatusOK, nil); err != nil {
		return err
	}

	return nil
}

package lambdaextensions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// LogType represents the type of logs in Lambda
type LogType string

const (
	// Platform is to receive logs emitted by the platform
	Platform LogType = "platform"
	// Function is to receive logs emitted by the function
	Function LogType = "function"
	// Extension is to receive logs emitted by the extension
	Extension LogType = "extension"
)

type SubEventType string

const (
	// RuntimeDone event is sent when lambda function is finished it's execution
	RuntimeDone SubEventType = "platform.runtimeDone"
)

// BufferingCfg is the configuration set for receiving logs from Logs API. Whichever of the conditions below is met first, the logs will be sent
type BufferingCfg struct {
	// MaxItems is the maximum number of events to be buffered in memory. (default: 10000, minimum: 1000, maximum: 10000)
	MaxItems uint32 `json:"maxItems"`
	// MaxBytes is the maximum size in bytes of the logs to be buffered in memory. (default: 262144, minimum: 262144, maximum: 1048576)
	MaxBytes uint32 `json:"maxBytes"`
	// TimeoutMS is the maximum time (in milliseconds) for a batch to be buffered. (default: 1000, minimum: 100, maximum: 30000)
	TimeoutMS uint32 `json:"timeoutMs"`
}

// HttpMethod represents the HTTP method used to receive logs from Logs API
type HttpMethod string

const (
	//HttpPost is to receive logs through POST.
	HttpPost HttpMethod = "POST"
	// HttpPut is to receive logs through PUT.
	HttpPut HttpMethod = "PUT"
)

// HttpProtocol is used to specify the protocol when subscribing to Logs API for HTTP
type HttpProtocol string

const (
	HttpProto HttpProtocol = "HTTP"
)

// HttpEncoding denotes what the content is encoded in
type HttpEncoding string

const (
	JSON HttpEncoding = "JSON"
)

// Destination is the configuration for listeners who would like to receive logs with HTTP
type Destination struct {
	Protocol   HttpProtocol `json:"protocol"`
	URI        string       `json:"URI"`
	HttpMethod HttpMethod   `json:"method,omitempty"`
	Encoding   HttpEncoding `json:"encoding,omitempty"`
}

type SchemaVersion string

const (
	SchemaVersion20210318 SchemaVersion = "2021-03-18"
)

// SubscribeRequest is the request body that is sent to Logs API on subscribe
type SubscribeRequest struct {
	SchemaVersion SchemaVersion `json:"schemaVersion,omitempty"`
	LogTypes      []LogType     `json:"types"`
	BufferingCfg  *BufferingCfg `json:"buffering,omitempty"`
	Destination   *Destination  `json:"destination"`
}

func NewSubscribeRequest(url string, logTypes []LogType) *SubscribeRequest {
	if len(logTypes) == 0 {
		logTypes = append(logTypes, Platform, Function, Extension)
	}
	return &SubscribeRequest{
		LogTypes: logTypes,
		Destination: &Destination{
			Protocol: HttpProto,
			URI:      url,
		},
	}
}

func (c *Client) Subscribe(ctx context.Context, subscribeReq *SubscribeRequest) error {
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

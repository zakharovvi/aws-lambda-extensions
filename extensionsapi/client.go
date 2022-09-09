package extensionsapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
)

// EventType represents the type of events received from /event/next
type EventType string

// ShutdownReason represents the reason for a shutdown event
type ShutdownReason string

const (
	// Invoke is the lambda invoke event
	Invoke EventType = "INVOKE"
	// Shutdown is a shutdown event for the environment
	Shutdown EventType = "SHUTDOWN"

	// nameHeader identifies the extension when registering
	nameHeader = "Lambda-Extension-Name"
	// idHeader is a uuid that is required on subsequent requests
	idHeader        = "Lambda-Extension-Identifier"
	errorTypeHeader = "Lambda-Extension-Function-Error-Type"

	// Spindown is a normal end to a function
	Spindown ShutdownReason = "spindown"
	// Timeout means the handler ran out of time
	Timeout ShutdownReason = "timeout"
	// Failure is any other shutdown type, such as out-of-memory
	Failure ShutdownReason = "failure"
)

type RegisterRequest struct {
	EventTypes []EventType `json:"events"`
}

// RegisterResponse is the body of the response for /register
type RegisterResponse struct {
	FunctionName    string            `json:"functionName"`
	FunctionVersion string            `json:"functionVersion"`
	Handler         string            `json:"handler"`
	Configuration   map[string]string `json:"configuration"` // todo what is inside?
}

// NextEventResponse is the response for /event/next
type NextEventResponse struct {
	// Either INVOKE or SHUTDOWN.
	EventType EventType `json:"eventType"`
	// The instant that the invocation times out, as epoch milliseconds
	DeadlineMs int64 `json:"deadlineMs"`
	// The AWS Request ID, for INVOKE events.
	RequestID string `json:"requestId"`
	// The ARN of the function being invoked, for INVOKE events.
	InvokedFunctionArn string `json:"invokedFunctionArn"`
	// XRay trace ID, for INVOKE events.
	Tracing Tracing `json:"tracing"`
	// The reason for termination, if this is a shutdown event
	ShutdownReason ShutdownReason `json:"shutdownReason"`
}

// Tracing is part of the response for /event/next
type Tracing struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// StatusResponse is the body of the response for /init/error and /exit/error
type StatusResponse struct {
	Status string `json:"status"`
}

type Client struct {
	baseURL     string
	httpClient  *http.Client
	extensionID string
}

func New(awsLambdaRuntimeAPI string) *Client {
	if awsLambdaRuntimeAPI == "" {
		awsLambdaRuntimeAPI = os.Getenv("AWS_LAMBDA_RUNTIME_API")
	}
	if awsLambdaRuntimeAPI == "" {
		panic("could not find extension API endpoint from environment variable AWS_LAMBDA_RUNTIME_API")
	}

	return &Client{
		baseURL:    fmt.Sprintf("http://%s/2020-01-01/extension", awsLambdaRuntimeAPI),
		httpClient: http.DefaultClient,
	}
}

// Register registers the extension with the Lambda Extensions API. This happens
// during Extension Init. Each call must include the list of events in the body
// and the extension name in the headers.
func (c *Client) Register(ctx context.Context, extensionName string, eventTypes []EventType) (*RegisterResponse, error) {
	if len(eventTypes) == 0 {
		eventTypes = append(eventTypes, Invoke, Shutdown)
	}
	if extensionName == "" {
		var err error
		extensionName, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("could not get full file name of the extension: %w", err)
		}
	}

	registerReq := RegisterRequest{EventTypes: eventTypes}
	body, err := json.Marshal(&registerReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/register", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set(nameHeader, extensionName)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("todo")
	}

	registerResp := &RegisterResponse{}
	if err := json.NewDecoder(resp.Body).Decode(registerResp); err != nil {
		return nil, err
	}

	c.extensionID = resp.Header.Get(idHeader)
	if c.extensionID == "" {
		return nil, err
	}

	return registerResp, nil
}

// NextEvent blocks while long polling for the next lambda invoke or shutdown
// By default, the Go HTTP client has no timeout, and in this case this is actually
// the desired behavior to enable long polling of the Extensions API.
func (c *Client) NextEvent(ctx context.Context) (*NextEventResponse, error) {
	if c.extensionID == "" {
		return nil, errors.New("not registered")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/event/next", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(idHeader, c.extensionID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("todo")
	}
	nextResp := &NextEventResponse{}
	if err := json.NewDecoder(resp.Body).Decode(nextResp); err != nil {
		return nil, err
	}
	return nextResp, nil
}

// InitError reports an initialization error to the platform. Call it when you registered but failed to initialize
func (c *Client) InitError(ctx context.Context, errorType string) (*StatusResponse, error) {
	return c.reportError(ctx, errorType, "/init/error")
}

// ExitError reports an error to the platform before exiting. Call it when you encounter an unexpected failure
func (c *Client) ExitError(ctx context.Context, errorType string) (*StatusResponse, error) {
	return c.reportError(ctx, errorType, "/exit/error")
}

func (c *Client) reportError(ctx context.Context, errorType, action string) (*StatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+action, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(idHeader, c.extensionID)
	req.Header.Set(errorTypeHeader, errorType)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s", resp.Status)
	}
	defer resp.Body.Close()
	statusResp := &StatusResponse{}

	if err := json.NewDecoder(resp.Body).Decode(statusResp); err != nil {
		return nil, err
	}
	return statusResp, nil
}

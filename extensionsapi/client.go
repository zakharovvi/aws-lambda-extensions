package extensionsapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
	Timeout ShutdownReason = "timout"
	// Failure is any other shutdown type, such as out-of-memory
	Failure ShutdownReason = "failure"
)

type RegisterRequest struct {
	EventTypes []EventType `json:"events"`
}

// RegisterResponse is the body of the response for /register
type RegisterResponse struct {
	FunctionName    string `json:"functionName"`
	FunctionVersion string `json:"functionVersion"`
	Handler         string `json:"handler"`
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

type ErrorRequest struct {
	ErrorMessage string   `json:"errorMessage"`
	ErrorType    string   `json:"errorType"`
	StackTrace   []string `json:"stackTrace"`
}

// ErrorResponse is the body of the response for /init/error and /exit/error
type ErrorResponse struct {
	Status string `json:"status"`
}

type options struct {
	extensionName       string
	awsLambdaRuntimeAPI string
	eventTypes          []EventType
	httpClient          *http.Client
}
type Option interface {
	apply(*options)
}

type extensionNameOption string

func (o extensionNameOption) apply(opts *options) {
	opts.extensionName = string(o)
}
func WithExtensionName(name string) Option {
	return extensionNameOption(name)
}

type awsLambdaRuntimeAPIOption string

func (o awsLambdaRuntimeAPIOption) apply(opts *options) {
	opts.extensionName = string(o)
}
func WithAWSLambdaRuntimeAPI(api string) Option {
	return awsLambdaRuntimeAPIOption(api)
}

type eventTypesOption []EventType

func (o eventTypesOption) apply(opts *options) {
	opts.eventTypes = o
}
func WithEventTypes(types []EventType) Option {
	return eventTypesOption(types)
}

type httpClientOption struct {
	httpClient *http.Client
}

func (o httpClientOption) apply(opts *options) {
	opts.httpClient = o.httpClient
}
func WithHTTPClient(httpClient *http.Client) Option {
	return httpClientOption{httpClient}
}

type Client struct {
	baseURL      string
	httpClient   *http.Client
	extensionID  string
	RegisterResp *RegisterResponse
}

// Register registers the extension with the Lambda Extensions API. This happens
// during Extension Init. Each call must include the list of events in the body
// and the extension name in the headers.
func Register(ctx context.Context, opts ...Option) (*Client, error) {
	extensionName, _ := os.Executable()
	extensionName = filepath.Base(extensionName)
	options := options{
		extensionName:       extensionName,
		awsLambdaRuntimeAPI: os.Getenv("AWS_LAMBDA_RUNTIME_API"),
		eventTypes:          []EventType{Invoke, Shutdown},
		httpClient:          http.DefaultClient,
	}
	for _, o := range opts {
		o.apply(&options)
	}
	if options.awsLambdaRuntimeAPI == "" {
		return nil, errors.New("could not find environment variable AWS_LAMBDA_RUNTIME_API")
	}

	client := &Client{
		baseURL:    fmt.Sprintf("http://%s/2020-01-01/extension", options.awsLambdaRuntimeAPI),
		httpClient: options.httpClient,
	}
	var err error
	client.RegisterResp, err = client.register(ctx, options.extensionName, options.eventTypes)
	if err != nil {
		return nil, fmt.Errorf("could not register extension: %w", err)
	}

	return client, nil
}

func (c *Client) register(ctx context.Context, extensionName string, eventTypes []EventType) (*RegisterResponse, error) {
	registerReq := RegisterRequest{eventTypes}
	body, err := json.Marshal(&registerReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/register", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set(nameHeader, extensionName)

	registerResp := &RegisterResponse{}
	resp, err := c.doRequest(req, http.StatusOK, registerResp)
	if err != nil {
		return nil, err
	}

	c.extensionID = resp.Header.Get(idHeader)
	if c.extensionID == "" {
		return nil, fmt.Errorf("could not find extension ID in register response header %s", idHeader)
	}

	return registerResp, nil
}

// NextEvent blocks while long polling for the next lambda invoke or shutdown
// By default, the Go HTTP client has no timeout, and in this case this is actually
// the desired behavior to enable long polling of the Extensions API.
func (c *Client) NextEvent(ctx context.Context) (*NextEventResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/event/next", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(idHeader, c.extensionID)

	nextResp := &NextEventResponse{}
	if _, err := c.doRequest(req, http.StatusOK, nextResp); err != nil {
		return nil, err
	}
	return nextResp, nil
}

// InitError reports an initialization error to the platform. Call it when you registered but failed to initialize
func (c *Client) InitError(ctx context.Context, errorType string, errorReq *ErrorRequest) (*ErrorResponse, error) {
	return c.reportError(ctx, errorType, "/init/error", errorReq)
}

// ExitError reports an error to the platform before exiting. Call it when you encounter an unexpected failure
func (c *Client) ExitError(ctx context.Context, errorType string, errorReq *ErrorRequest) (*ErrorResponse, error) {
	return c.reportError(ctx, errorType, "/exit/error", errorReq)
}

func (c *Client) reportError(ctx context.Context, errorType, action string, errorReq *ErrorRequest) (*ErrorResponse, error) {
	var body []byte
	if errorReq != nil {
		var err error
		if body, err = json.Marshal(errorReq); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+action, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set(idHeader, c.extensionID)
	req.Header.Set(errorTypeHeader, errorType)

	errorResp := &ErrorResponse{}
	if _, err := c.doRequest(req, http.StatusAccepted, errorResp); err != nil {
		return nil, err
	}
	return errorResp, nil
}

func (c *Client) doRequest(req *http.Request, wantStatus int, out interface{}) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		return nil, fmt.Errorf("request failed with status %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, err
	}
	return resp, nil
}

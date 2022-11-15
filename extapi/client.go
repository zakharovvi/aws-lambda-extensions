package extapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	lambdaext "github.com/zakharovvi/aws-lambda-extensions"
)

// EventType represents the type of events received from /event/next.
type EventType string

const (
	// Invoke is the lambda invoke event.
	Invoke EventType = "INVOKE"
	// Shutdown is a shutdown event for the environment.
	Shutdown EventType = "SHUTDOWN"
)

// ShutdownReason represents the reason for a shutdown event.
type ShutdownReason string

const (
	// Spindown is a normal end to a function.
	Spindown ShutdownReason = "spindown"
	// Timeout means the handler ran out of time.
	Timeout ShutdownReason = "timout"
	// Failure is any other shutdown type, such as out-of-memory.
	Failure ShutdownReason = "failure"
	// ExtensionError is used when one of Client or Extension methods return error. It is not returned by lambda.
	ExtensionError ShutdownReason = "extension_error"
)

type RegisterRequest struct {
	EventTypes []EventType `json:"events"`
}

// RegisterResponse is the body of the response for /register.
type RegisterResponse struct {
	FunctionName    string                    `json:"functionName"`
	FunctionVersion lambdaext.FunctionVersion `json:"functionVersion"`
	Handler         string                    `json:"handler"`
	AccountID       string                    `json:"accountId"`
}

// NextEventResponse is the response for /event/next.
type NextEventResponse struct {
	// Either INVOKE or SHUTDOWN.
	EventType EventType `json:"eventType"`
	// The instant that the invocation times out, as epoch milliseconds.
	DeadlineMs int64 `json:"deadlineMs"`
	// The AWS Request ID, for INVOKE events.
	RequestID lambdaext.RequestID `json:"requestId"`
	// The ARN of the function being invoked, for INVOKE events.
	InvokedFunctionArn string `json:"invokedFunctionArn"`
	// XRay trace ID, for INVOKE events.
	Tracing Tracing `json:"tracing"`
	// The reason for termination, if this is a shutdown event.
	ShutdownReason ShutdownReason `json:"shutdownReason"`
}

// Tracing is part of the response for /event/next.
type Tracing struct {
	Type  lambdaext.TracingType  `json:"type"`
	Value lambdaext.TracingValue `json:"value"`
}

// ErrorResponse is the body of the response for /init/error and /exit/error.
type ErrorResponse struct {
	Status string `json:"status"`
}

const (
	// nameHeader identifies the extension when registering.
	nameHeader = "Lambda-Extension-Name"
	// idHeader is a uuid that is required on subsequent requests.
	idHeader        = "Lambda-Extension-Identifier"
	errorTypeHeader = "Lambda-Extension-Function-Error-Type"
	// acceptFeatureHeader is used to specify optional Extensions features during registration.
	acceptFeatureHeader = "Lambda-Extension-Accept-Feature"
)

type LambdaAPIError struct {
	Type           string `json:"errorType"`
	Message        string `json:"errorMessage"`
	HTTPStatusCode int    `json:"-"`
}

func (e LambdaAPIError) Error() string {
	return fmt.Sprintf("Lambda API http_status_code=%d type=%s, message=%s", e.HTTPStatusCode, e.Type, e.Message)
}

type options struct {
	extensionName       lambdaext.ExtensionName
	awsLambdaRuntimeAPI string
	eventTypes          []EventType
	httpClient          *http.Client
	log                 logr.Logger
}
type Option interface {
	apply(*options)
}

type extensionNameOption lambdaext.ExtensionName

func (o extensionNameOption) apply(opts *options) {
	opts.extensionName = lambdaext.ExtensionName(o)
}

func WithExtensionName(name lambdaext.ExtensionName) Option {
	return extensionNameOption(name)
}

type awsLambdaRuntimeAPIOption string

func (o awsLambdaRuntimeAPIOption) apply(opts *options) {
	opts.awsLambdaRuntimeAPI = string(o)
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

type loggerOption struct {
	log logr.Logger
}

func (o loggerOption) apply(opts *options) {
	opts.log = o.log
}

func WithLogger(log logr.Logger) Option {
	return loggerOption{log}
}

// Client is a Low-level Lambda API client.
// In most situations it's better to use high-level handlers extapi.Run and logsapi.Run.
type Client struct {
	runtimeAPI   string
	httpClient   *http.Client
	extensionID  string
	registerResp *RegisterResponse
	log          logr.Logger
}

func (c *Client) FunctionName() string {
	return c.registerResp.FunctionName
}

func (c *Client) FunctionVersion() lambdaext.FunctionVersion {
	return c.registerResp.FunctionVersion
}

func (c *Client) Handler() string {
	return c.registerResp.Handler
}

// AccountID returns the account ID associated with the Lambda function that you're registering the extension for.
func (c *Client) AccountID() string {
	return c.registerResp.AccountID
}

func (c *Client) ExtensionID() string {
	return c.extensionID
}

// Register registers the extension with the Lambda Extensions API. This happens
// during extension Init. Each call must include the list of events in the body
// and the lambdaext.ExtensionName in the headers.
func Register(ctx context.Context, opts ...Option) (*Client, error) {
	extensionName, _ := os.Executable()
	extensionName = filepath.Base(extensionName)
	options := options{
		extensionName:       lambdaext.ExtensionName(extensionName),
		awsLambdaRuntimeAPI: EnvAWSLambdaRuntimeAPI(),
		eventTypes:          []EventType{Invoke, Shutdown},
		httpClient:          http.DefaultClient,
		log:                 logr.FromContextOrDiscard(ctx),
	}
	for _, o := range opts {
		o.apply(&options)
	}
	if options.awsLambdaRuntimeAPI == "" {
		err := errors.New("could not find environment variable AWS_LAMBDA_RUNTIME_API")
		options.log.Error(err, "")

		return nil, err
	}
	options.log.V(1).Info("using AWS_LAMBDA_RUNTIME_API", "addr", options.awsLambdaRuntimeAPI)

	client := &Client{
		runtimeAPI: options.awsLambdaRuntimeAPI,
		httpClient: options.httpClient,
		log:        options.log,
	}
	var err error
	client.registerResp, err = client.register(ctx, options.extensionName, options.eventTypes)
	if err != nil {
		err = fmt.Errorf("could not register extension: %w", err)
		options.log.Error(err, "")

		return nil, err
	}

	client.log.V(1).Info("extension registered", "extensionID", client.extensionID)

	return client, nil
}

func (c *Client) register(ctx context.Context, extensionName lambdaext.ExtensionName, eventTypes []EventType) (*RegisterResponse, error) {
	registerReq := RegisterRequest{eventTypes}
	body, err := json.Marshal(&registerReq)
	if err != nil {
		return nil, fmt.Errorf("could not json encode register request: %w", err)
	}
	c.log.V(1).Info("sending register request", "body", string(body))

	url := fmt.Sprintf("http://%s/2020-01-01/extension/register", c.runtimeAPI)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create register http request: %w", err)
	}
	req.Header.Set(nameHeader, string(extensionName))
	req.Header.Set(acceptFeatureHeader, "accountId")

	registerResp := &RegisterResponse{}
	resp, err := c.doRequest(req, http.StatusOK, registerResp)
	if err != nil {
		return nil, fmt.Errorf("register http call failed: %w", err)
	}

	c.extensionID = resp.Header.Get(idHeader)
	if c.extensionID == "" {
		return nil, fmt.Errorf("could not find extension ID in register response header %s", idHeader)
	}

	c.log.V(1).Info("received register response", "response", registerResp)

	return registerResp, nil
}

// NextEvent blocks while long polling for the next lambda invoke or shutdown
// By default, the Go HTTP client has no timeout, and in this case this is actually
// the desired behavior to enable long polling of the Extensions API.
func (c *Client) NextEvent(ctx context.Context) (*NextEventResponse, error) {
	c.log.V(1).Info("requesting event/next")
	url := fmt.Sprintf("http://%s/2020-01-01/extension/event/next", c.runtimeAPI)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		err = fmt.Errorf("could not create http request for event/next: %w", err)
		c.log.Error(err, "")

		return nil, err
	}

	nextResp := &NextEventResponse{}
	if _, err := c.doRequest(req, http.StatusOK, nextResp); err != nil {
		err = fmt.Errorf("event/next call failed: %w", err)
		c.log.Error(err, "")

		return nil, err
	}
	c.log.V(1).Info("event/next response received", "event", nextResp)

	return nextResp, nil
}

// InitError reports an initialization error to the platform. Call it when you registered but failed to initialize.
func (c *Client) InitError(ctx context.Context, errorType string, err error) (*ErrorResponse, error) {
	return c.reportError(ctx, "/init/error", errorType, err)
}

// ExitError reports an error to the platform before exiting. Call it when you encounter an unexpected failure.
func (c *Client) ExitError(ctx context.Context, errorType string, err error) (*ErrorResponse, error) {
	return c.reportError(ctx, "/exit/error", errorType, err)
}

func (c *Client) reportError(ctx context.Context, action, errorType string, err error) (*ErrorResponse, error) {
	c.log.V(1).Info("reporting error", "action", action, "errorType", errorType, "body", err.Error())
	url := fmt.Sprintf("http://%s/2020-01-01/extension%s", c.runtimeAPI, action)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(err.Error()))
	if err != nil {
		err = fmt.Errorf("could not create http request for error reporting %s: %w", action, err)
		c.log.Error(err, "")

		return nil, err
	}
	req.Header.Set(errorTypeHeader, errorType)

	errorResp := &ErrorResponse{}
	if _, err := c.doRequest(req, http.StatusAccepted, errorResp); err != nil {
		err = fmt.Errorf("error reporting %s call failed: %w", action, err)
		c.log.Error(err, "")

		return nil, err
	}
	c.log.V(1).Info("error has been reported", "action", action, "response", errorResp)

	return errorResp, nil
}

func (c *Client) doRequest(req *http.Request, wantStatus int, out interface{}) (*http.Response, error) {
	if req.Method == http.MethodPost || req.Method == http.MethodPut || req.Method == http.MethodPatch {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.extensionID != "" {
		req.Header.Set(idHeader, c.extensionID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.Error(err, "could not close http response body")
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read http response body: %w", err)
	}
	if resp.StatusCode != wantStatus {
		apiErr := LambdaAPIError{}
		apiErr.HTTPStatusCode = resp.StatusCode
		if err := json.Unmarshal(body, &apiErr); err != nil {
			return nil, fmt.Errorf("http request failed with status %s and body: %s", resp.Status, body)
		}

		return nil, apiErr
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return nil, fmt.Errorf("could not json decode http response %s: %w", body, err)
		}
	}

	return resp, nil
}

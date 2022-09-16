package lambdaextensions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LogSubscriptionType represents the type of logs in Lambda
type LogSubscriptionType string

const (
	// Platform is to receive logs emitted by the platform
	Platform LogSubscriptionType = "platform"
	// Function is to receive logs emitted by the function
	Function LogSubscriptionType = "function"
	// Extension is to receive logs emitted by the extension
	Extension LogSubscriptionType = "extension"
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
	SchemaVersion SchemaVersion         `json:"schemaVersion,omitempty"`
	LogTypes      []LogSubscriptionType `json:"types"`
	BufferingCfg  *BufferingCfg         `json:"buffering,omitempty"`
	Destination   *Destination          `json:"destination"`
}

func NewSubscribeRequest(url string, logTypes []LogSubscriptionType) *SubscribeRequest {
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

type LogType string

const (
	LogPlatformStart            LogType = "platform.start"
	LogPlatformEnd              LogType = "platform.end"
	LogPlatformReport           LogType = "platform.report"
	LogPlatformExtension        LogType = "platform.extension"
	LogPlatformLogsSubscription LogType = "platform.logsSubscription"
	LogPlatformLogsDropped      LogType = "platform.logsDropped"
	LogPlatformFault            LogType = "platform.fault"
	LogPlatformRuntimeDone      LogType = "platform.runtimeDone"
	LogFunction                 LogType = "function"
	LogExtension                LogType = "extension"
)

type Log struct {
	LogType   LogType         `json:"type"`
	Time      time.Time       `json:"time"`
	RawRecord json.RawMessage `json:"record"`
	Record    any
}

type PlatformStartRecord struct {
	RequestID string `json:"requestId"`
	Version   string `json:"version,omitempty"`
}

type PlatformEndRecord struct {
	RequestID string `json:"requestId"`
}

type PlatformReportRecord struct {
	Metrics   Metrics `json:"metrics"`
	RequestID string  `json:"requestId"`
	Tracing   Tracing `json:"tracing,omitempty"`
}
type Metrics struct {
	DurationMs       float64 `json:"durationMs"`
	BilledDurationMs float64 `json:"billedDurationMs"`
	MemorySizeMB     uint64  `json:"memorySizeMB"`
	MaxMemoryUsedMB  uint64  `json:"maxMemoryUsedMB"`
	InitDurationMs   float64 `json:"initDurationMs"`
}

type PlatformExtensionRecord struct {
	Events []EventType `json:"events"`
	Name   string      `json:"name"`
	State  string      `json:"state"`
}

type PlatformLogsSubscriptionRecord struct {
	Name  string                `json:"name"`
	State string                `json:"state"`
	Types []LogSubscriptionType `json:"types"`
}

type PlatformLogsDroppedRecord struct {
	DroppedBytes   uint64 `json:"droppedBytes"`
	DroppedRecords uint64 `json:"droppedRecords"`
	Reason         string `json:"reason"`
}

type PlatformFaultRecord string

type PlatformRuntimeDoneRecord struct {
	RequestID string            `json:"requestId"`
	Status    RuntimeDoneStatus `json:"status"`
}
type RuntimeDoneStatus string

const (
	RuntimeDoneSuccess RuntimeDoneStatus = "success"
	RuntimeDoneFailure RuntimeDoneStatus = "failure"
	RuntimeDoneTimeout RuntimeDoneStatus = "timeout"
)

type FunctionRecord string

type ExtensionRecord string

// DecodeLogs consumes all logs from json array stream and close it afterwards
func DecodeLogs(r io.ReadCloser, logs chan<- Log) error {
	defer func() {
		_, _ = io.Copy(io.Discard, r)
		_ = r.Close()
	}()

	d := json.NewDecoder(r)
	if err := readBracket(d, "["); err != nil {
		return err
	}
	for d.More() {
		msg := Log{}
		if err := d.Decode(&msg); err != nil {
			return err
		}
		switch msg.LogType {
		case LogPlatformStart:
			msg.Record = new(PlatformStartRecord)
		case LogPlatformEnd:
			msg.Record = new(PlatformEndRecord)
		case LogPlatformReport:
			msg.Record = new(PlatformReportRecord)
		case LogPlatformExtension:
			msg.Record = new(PlatformExtensionRecord)
		case LogPlatformLogsSubscription:
			msg.Record = new(PlatformLogsSubscriptionRecord)
		case LogPlatformLogsDropped:
			msg.Record = new(PlatformLogsDroppedRecord)
		case LogPlatformFault:
			msg.Record = new(PlatformFaultRecord)
		case LogPlatformRuntimeDone:
			msg.Record = new(PlatformRuntimeDoneRecord)
		case LogFunction:
			msg.Record = new(FunctionRecord)
		case LogExtension:
			msg.Record = new(ExtensionRecord)
		default:
			return fmt.Errorf(`could not decode unknown log type "%s" and record "%s"`, msg.LogType, msg.RawRecord)
		}
		if err := json.Unmarshal(msg.RawRecord, msg.Record); err != nil {
			return fmt.Errorf("could not unmarshal record %s for log type %s with error: %w", msg.RawRecord, msg.LogType, err)
		}
		logs <- msg
	}
	if err := readBracket(d, "]"); err != nil {
		return err
	}
	return nil
}

func readBracket(d *json.Decoder, want string) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return fmt.Errorf("malformed json array, want %s, got %v", want, t)
	}
	if delim.String() != want {
		return fmt.Errorf("malformed json array, want %s, got %v", want, delim.String())
	}
	return nil
}

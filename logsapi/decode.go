package logsapi

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/zakharovvi/lambda-extensions/extapi"
)

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

type RecordPlatformStart struct {
	RequestID string `json:"requestId"`
	Version   string `json:"version,omitempty"`
}

type RecordPlatformEnd struct {
	RequestID string `json:"requestId"`
}

type RecordPlatformReport struct {
	Metrics   Metrics        `json:"metrics"`
	RequestID string         `json:"requestId"`
	Tracing   extapi.Tracing `json:"tracing,omitempty"`
}
type Metrics struct {
	DurationMs       float64 `json:"durationMs"`
	BilledDurationMs float64 `json:"billedDurationMs"`
	MemorySizeMB     uint64  `json:"memorySizeMB"`
	MaxMemoryUsedMB  uint64  `json:"maxMemoryUsedMB"`
	InitDurationMs   float64 `json:"initDurationMs"`
}

type RecordPlatformExtension struct {
	Events []extapi.EventType `json:"events"`
	Name   string             `json:"name"`
	State  string             `json:"state"`
}

type RecordPlatformLogsSubscription struct {
	Name  string                       `json:"name"`
	State string                       `json:"state"`
	Types []extapi.LogSubscriptionType `json:"types"`
}

type RecordPlatformLogsDropped struct {
	DroppedBytes   uint64 `json:"droppedBytes"`
	DroppedRecords uint64 `json:"droppedRecords"`
	Reason         string `json:"reason"`
}

type RecordPlatformFault string

type RecordPlatformRuntimeDone struct {
	RequestID string            `json:"requestId"`
	Status    RuntimeDoneStatus `json:"status"`
}
type RuntimeDoneStatus string

const (
	RuntimeDoneSuccess RuntimeDoneStatus = "success"
	RuntimeDoneFailure RuntimeDoneStatus = "failure"
	RuntimeDoneTimeout RuntimeDoneStatus = "timeout"
)

type RecordFunction string

type RecordExtension string

// DecodeLogs consumes all logs from json array stream and close it afterwards.
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
			return fmt.Errorf("could not decode log message from json array: %w", err)
		}
		switch msg.LogType {
		case LogPlatformStart:
			msg.Record = new(RecordPlatformStart)
		case LogPlatformEnd:
			msg.Record = new(RecordPlatformEnd)
		case LogPlatformReport:
			msg.Record = new(RecordPlatformReport)
		case LogPlatformExtension:
			msg.Record = new(RecordPlatformExtension)
		case LogPlatformLogsSubscription:
			msg.Record = new(RecordPlatformLogsSubscription)
		case LogPlatformLogsDropped:
			msg.Record = new(RecordPlatformLogsDropped)
		case LogPlatformFault:
			msg.Record = new(RecordPlatformFault)
		case LogPlatformRuntimeDone:
			msg.Record = new(RecordPlatformRuntimeDone)
		case LogFunction:
			msg.Record = new(RecordFunction)
		case LogExtension:
			msg.Record = new(RecordExtension)
		default:
			return fmt.Errorf(`could not decode unknown log type "%s" and record "%s"`, msg.LogType, msg.RawRecord)
		}
		if err := json.Unmarshal(msg.RawRecord, msg.Record); err != nil {
			return fmt.Errorf("could not decode log record %s for log type %s with error: %w", msg.RawRecord, msg.LogType, err)
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
		return fmt.Errorf("malformed json array: %w", err)
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

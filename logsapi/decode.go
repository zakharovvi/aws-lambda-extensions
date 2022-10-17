package logsapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/zakharovvi/aws-lambda-extensions/extapi"
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
	Record    any             `json:"-"`
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
func DecodeLogs(ctx context.Context, r io.ReadCloser, logs chan<- Log) error {
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
		var unmarshalErr error
		switch msg.LogType {
		case LogPlatformStart:
			record := RecordPlatformStart{}
			unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
			msg.Record = record
		case LogPlatformEnd:
			record := RecordPlatformEnd{}
			unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
			msg.Record = record
		case LogPlatformReport:
			record := RecordPlatformReport{}
			unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
			msg.Record = record
		case LogPlatformExtension:
			record := RecordPlatformExtension{}
			unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
			msg.Record = record
		case LogPlatformLogsSubscription:
			record := RecordPlatformLogsSubscription{}
			unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
			msg.Record = record
		case LogPlatformLogsDropped:
			record := RecordPlatformLogsDropped{}
			unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
			msg.Record = record
		case LogPlatformFault:
			record := RecordPlatformFault("")
			unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
			msg.Record = record
		case LogPlatformRuntimeDone:
			record := RecordPlatformRuntimeDone{}
			unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
			msg.Record = record
		case LogFunction:
			record := RecordFunction("")
			unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
			msg.Record = record
		case LogExtension:
			record := RecordExtension("")
			unmarshalErr = json.Unmarshal(msg.RawRecord, &record)
			msg.Record = record
		default:
			return fmt.Errorf(`could not decode unknown log type "%s" and record "%s"`, msg.LogType, msg.RawRecord)
		}
		if unmarshalErr != nil {
			return fmt.Errorf("could not decode log record %s for log type %s with error: %w", msg.RawRecord, msg.LogType, unmarshalErr)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("decoding was interrupted with context error: %w", ctx.Err())
		default:
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

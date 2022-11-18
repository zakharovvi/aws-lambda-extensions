// Sample extension to demonstrate how to use Lambda Telemetry API.
package main

import (
	"context"
	"log"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi"
)

type TelemetryProcessor struct {
	logger logr.Logger
}

func (proc *TelemetryProcessor) Init(ctx context.Context, client *extapi.Client) error {
	proc.logger.Info(
		"initializing event processor...",
		"FunctionName", client.FunctionName(),
		"FunctionVersion", client.FunctionVersion(),
		"handler", client.Handler(),
		"accountID", client.AccountID(),
	)

	return nil
}

func (proc *TelemetryProcessor) Process(ctx context.Context, msg telemetryapi.Event) error {
	msg.RawRecord = nil // do not log raw bytes
	proc.logger.Info(
		"received an event",
		"msg", msg,
	)
	return nil
}

func (proc *TelemetryProcessor) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	proc.logger.Info(
		"shutting down event processor...",
		"reason", reason,
		"error", err,
	)

	return nil
}

func main() {
	// log library debug messages
	stdr.SetVerbosity(1)
	logger := stdr.New(log.New(os.Stdout, "", log.Lshortfile))

	if err := telemetryapi.Run(
		context.Background(),
		&TelemetryProcessor{logger},
		telemetryapi.WithLogger(logger),
		telemetryapi.WithBufferingCfg(&extapi.TelemetryBufferingCfg{TimeoutMS: 25, MaxBytes: 262144, MaxItems: 1000}),
	); err != nil {
		log.Panic(err)
	}
}

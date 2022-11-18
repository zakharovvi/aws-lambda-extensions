// Sample extension to demonstrate how to use Lambda Logs API.
package main

import (
	"context"
	"log"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/logsapi"
)

type Processor struct {
	logger logr.Logger
}

func (proc *Processor) Init(ctx context.Context, client *extapi.Client) error {
	proc.logger.Info(
		"initializing log processor...",
		"FunctionName", client.FunctionName(),
		"FunctionVersion", client.FunctionVersion(),
		"handler", client.Handler(),
		"accountID", client.AccountID(),
	)

	return nil
}

func (proc *Processor) Process(ctx context.Context, msg logsapi.Log) error {
	msg.RawRecord = nil // do not log raw bytes
	proc.logger.Info(
		"received log message",
		"msg", msg,
	)
	return nil
}

func (proc *Processor) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	proc.logger.Info(
		"shutting down Processor...",
		"reason", reason,
		"error", err,
	)

	return nil
}

func main() {
	// log library debug messages
	stdr.SetVerbosity(1)
	logger := stdr.New(log.New(os.Stdout, "", log.Lshortfile))

	if err := logsapi.Run(
		context.Background(),
		&Processor{logger},
		logsapi.WithLogger(logger),
		logsapi.WithBufferingCfg(&extapi.LogsBufferingCfg{TimeoutMS: 25, MaxBytes: 262144, MaxItems: 1000}),
	); err != nil {
		log.Panic(err)
	}
}

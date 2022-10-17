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

type LogProcessor struct {
	logger logr.Logger
}

func (l *LogProcessor) Process(ctx context.Context, msg logsapi.Log) error {
	msg.RawRecord = nil // do not log raw bytes
	l.logger.Info(
		"received log message",
		"msg", msg,
	)
	return nil
}

func (l *LogProcessor) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	l.logger.Info(
		"shutting down LogProcessor...",
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
		&LogProcessor{logger},
		logsapi.WithLogger(logger),
		logsapi.WithBufferingCfg(&extapi.LogsBufferingCfg{TimeoutMS: 25, MaxBytes: 262144, MaxItems: 1000}),
	); err != nil {
		log.Panic(err)
	}
}

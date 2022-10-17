package logsapi_test

import (
	"context"
	"log"

	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/logsapi"
)

type LogProcessor struct{}

func (l *LogProcessor) Process(ctx context.Context, msg logsapi.Log) error {
	log.Printf("time=%s type=%s\n", msg.LogType, msg.Time)

	return nil
}

func (l *LogProcessor) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	log.Printf("shutting down extension due to reason=%s error=%v\n", reason, err)

	return nil
}

func ExampleRun() {
	if err := logsapi.Run(context.Background(), &LogProcessor{}); err != nil {
		log.Panic(err)
	}
}

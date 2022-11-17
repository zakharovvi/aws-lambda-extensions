package logsapi_test

import (
	"context"
	"log"

	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/logsapi"
)

type LogProcessor struct{}

func (lp *LogProcessor) Init(ctx context.Context, client *extapi.Client) error {
	log.Printf(
		"initializing LogProcessor for function %s(%s), handler %s and accountID %s\n",
		client.FunctionName(),
		client.FunctionVersion(),
		client.Handler(),
		client.AccountID(),
	)

	return nil
}

func (lp *LogProcessor) Process(ctx context.Context, msg logsapi.Log) error {
	log.Printf("time=%s type=%s\n", msg.LogType, msg.Time)

	return nil
}

func (lp *LogProcessor) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	log.Printf("shutting down extension due to reason=%s error=%v\n", reason, err)

	return nil
}

func ExampleRun() {
	if err := logsapi.Run(context.Background(), &LogProcessor{}); err != nil {
		log.Panic(err)
	}
}

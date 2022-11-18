package telemetryapi_test

import (
	"context"
	"log"

	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi"
)

type Processor struct{}

func (proc *Processor) Init(ctx context.Context, client *extapi.Client) error {
	log.Printf(
		"initializing Processor for function %s(%s), handler %s, and accountID %s\n",
		client.FunctionName(),
		client.FunctionVersion(),
		client.Handler(),
		client.AccountID(),
	)

	return nil
}

func (proc *Processor) Process(ctx context.Context, msg telemetryapi.Event) error {
	log.Printf("time=%s type=%s\n", msg.Type, msg.Time)

	return nil
}

func (proc *Processor) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	log.Printf("shutting down extension due to reason=%s error=%v\n", reason, err)

	return nil
}

func ExampleRun() {
	if err := telemetryapi.Run(context.Background(), &Processor{}); err != nil {
		log.Panic(err)
	}
}

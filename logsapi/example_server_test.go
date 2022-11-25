package logsapi_test

import (
	"context"
	"log"

	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/logsapi"
)

type Processor struct{}

func (proc *Processor) Init(ctx context.Context, registerResp *extapi.RegisterResponse) error {
	log.Printf(
		"initializing Processor for function %s(%s), handler %s and accountID %s\n",
		registerResp.FunctionName,
		registerResp.FunctionVersion,
		registerResp.Handler,
		registerResp.AccountID,
	)

	return nil
}

func (proc *Processor) Process(ctx context.Context, msg logsapi.Log) error {
	log.Printf("time=%s type=%s\n", msg.LogType, msg.Time)

	return nil
}

func (proc *Processor) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	log.Printf("shutting down extension due to reason=%s error=%v\n", reason, err)

	return nil
}

func ExampleRun() {
	if err := logsapi.Run(context.Background(), &Processor{}); err != nil {
		log.Panic(err)
	}
}

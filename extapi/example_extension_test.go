package extapi_test

import (
	"context"
	"encoding/json"
	"log"

	"github.com/zakharovvi/aws-lambda-extensions/extapi"
)

type Extension struct{}

func (ext *Extension) Init(ctx context.Context, client *extapi.Client) error {
	log.Printf(
		"initializing extension for function %s(%s) and handler %s\n",
		client.FunctionName(),
		client.FunctionVersion(),
		client.Handler(),
	)

	return nil
}

func (ext *Extension) HandleInvokeEvent(ctx context.Context, event *extapi.NextEventResponse) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	log.Printf("received invocation event: %s\n", b)

	return nil
}

func (ext *Extension) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	log.Printf("shutting down extension due to reason=%s error=%v\n", reason, err)

	return nil
}

func (ext *Extension) Err() <-chan error {
	return nil
}

func ExampleRun() {
	if err := extapi.Run(context.Background(), &Extension{}); err != nil {
		log.Panic(err)
	}
}

package extapi_test

import (
	"context"
	"encoding/json"
	"log"

	"github.com/zakharovvi/aws-lambda-extensions/extapi"
)

type InvokeExtension struct{}

func (ext *InvokeExtension) Init(ctx context.Context, client *extapi.Client) error {
	log.Printf("initializing extension for function %s(%s) and handler %s\n", client.FunctionName(), client.FunctionVersion(), client.Handler())

	return nil
}

func (ext *InvokeExtension) HandleInvokeEvent(ctx context.Context, event *extapi.NextEventResponse) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	log.Printf("received invocation event: %s\n", b)

	return nil
}

func (ext *InvokeExtension) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	log.Printf("shutting down extension due to reason=%s error=%v\n", reason, err)

	return nil
}

func (ext *InvokeExtension) Err() <-chan error {
	return nil
}

func Example_invoke() {
	if err := extapi.Run(context.Background(), &InvokeExtension{}); err != nil {
		log.Panic(err)
	}
}

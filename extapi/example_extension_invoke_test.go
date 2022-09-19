package extapi_test

import (
	"context"
	"encoding/json"
	"log"

	"github.com/zakharovvi/lambda-extensions/extapi"
)

type InvokeExtension struct{}

func (s *InvokeExtension) Init(ctx context.Context, client *extapi.Client) error {
	log.Printf("initializing extension for function %s(%s) and handler %s\n", client.FunctionName(), client.FunctionVersion(), client.Handler())
	return nil
}

func (s *InvokeExtension) HandleInvokeEvent(ctx context.Context, event *extapi.NextEventResponse) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	log.Printf("received invokation event: %s\n", b)
	return nil
}

func (s *InvokeExtension) Shutdown(ctx context.Context, reason extapi.ShutdownReason) error {
	log.Printf("shutting down extension due to : %s\n", reason)
	return nil
}

func Example_invoke() {
	ext := &InvokeExtension{}
	if err := extapi.Run(context.Background(), ext); err != nil {
		log.Fatal(err)
	}
}

package extapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/zakharovvi/lambda-extensions/extapi"
)

type InvokeExtension struct{}

func (s *InvokeExtension) Init(ctx context.Context, client *extapi.Client) error {
	fmt.Printf("initializing extension for function %s(%s) and handler %s\n", client.FunctionName(), client.FunctionVersion(), client.Handler())
	return nil
}

func (s *InvokeExtension) Invoke(ctx context.Context, event *extapi.NextEventResponse) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	fmt.Printf("received invokation event: %s", b)
	return nil
}

func (s *InvokeExtension) Shutdown(ctx context.Context) error {
	fmt.Println("shutting down extension")
	return nil
}

func Example_invoke() {
	ext := &InvokeExtension{}
	if err := extapi.Run(context.Background(), ext); err != nil {
		log.Fatal(err)
	}
}

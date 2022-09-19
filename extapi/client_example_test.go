package extapi_test

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/zakharovvi/lambda-extensions/extapi"
)

// End to end example how to use Client, process events and handle errors
func ExampleClient() {
	ctx := context.Background()

	// 1. register extension
	client, err := extapi.Register(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println(client.FunctionName())
	log.Println(client.FunctionVersion())
	log.Println(client.Handler())

	// 2. initialize the extension
	initFunc := func() error { return nil }
	if err := initFunc(); err != nil {
		// report error and exit if initialization failed
		_, _ = client.InitError(ctx, "ExtensionName.Reason", err)
		log.Fatalln(err)
	}

	// 3. start polling events
	// first NextEvent calls notifies lambda that extension initialization has finished
	for {
		event, err := client.NextEvent(ctx)
		if err != nil {
			// report error and exit if event processing failed
			_, _ = client.ExitError(ctx, "ExtensionName.Reason", err)
			log.Fatalln(err)
		}
		if event.EventType == extapi.Shutdown {
			log.Println(event.ShutdownReason)
			os.Exit(0)
		}

		processEventFunc := func(event *extapi.NextEventResponse) error { return nil }
		if err := processEventFunc(event); err != nil {
			// 4. report error and exit if event processing failed
			_, _ = client.ExitError(ctx, "ExtensionName.Reason", err)
			log.Fatalln(err)
		}
	}
}

// Register supports optional arguments to override defaults
func ExampleRegister() {
	ctx := context.Background()

	client, err := extapi.Register(
		ctx,
		extapi.WithEventTypes([]extapi.EventType{extapi.Shutdown}),
		extapi.WithExtensionName("binary_file_basename"),
		extapi.WithAWSLambdaRuntimeAPI("127.0.0.1:8080"),
		extapi.WithHTTPClient(http.DefaultClient),
	)
	if err != nil {
		log.Fatalln(err)
	}
	_ = client
}

// Client.ExitError and Client.InitError accept error details to send to lambda
func ExampleClient_ExitError() {
	ctx := context.Background()

	client, err := extapi.Register(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	errResp, err := client.ExitError(ctx, "ExtensionName.Reason", errors.New("text description of the error"))
	if err != nil {
		log.Println(err)
	}
	if errResp.Status != "OK" {
		log.Printf("unknown error response status: %s, want OK", errResp.Status)
	}
}

func ExampleClient_LogsSubscribe() {
	ctx := context.Background()

	// 1. register extension and subscribe only to shutdown events
	client, err := extapi.Register(ctx, extapi.WithEventTypes([]extapi.EventType{extapi.Shutdown}))
	if err != nil {
		log.Fatalln(err)
	}

	// 2. start log receiving server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// process logs
	}))
	defer server.Close()

	// 3. subscribe to logs api
	req := extapi.NewLogsSubscribeRequest(server.URL, nil)
	if err := client.LogsSubscribe(ctx, req); err != nil {
		// 4. report error and exit if event processing failed
		_, _ = client.ExitError(ctx, "ExtensionName.Reason", err)
		log.Fatalln(err)
	}

	// 5. wait for shutdown event
	for {
		_, _ = client.NextEvent(ctx)
	}
}

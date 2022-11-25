package extapi_test

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/zakharovvi/aws-lambda-extensions/extapi"
)

// End to end example how to use Client, process events and handle errors.
// Please consider using Run function which is a high-level wrapper over Client.
func ExampleClient() {
	ctx := context.Background()

	// 1. register extension
	client, err := extapi.Register(ctx)
	if err != nil {
		log.Panic(err)
	}
	registerResp := client.GetRegisterResponse()
	log.Println(registerResp.FunctionName)
	log.Println(registerResp.FunctionVersion)
	log.Println(registerResp.Handler)
	log.Println(registerResp.AccountID)

	// 2. initialize the extension
	initFunc := func() error { return nil }
	if err := initFunc(); err != nil {
		// report error and exit if initialization failed
		_, _ = client.InitError(ctx, "ExtensionName.Reason", err)
		log.Panic(err)
	}

	// 3. start polling events
	// first NextEvent calls notifies lambda that extension initialization has finished
	for {
		event, err := client.NextEvent(ctx)
		if err != nil {
			// report error and exit if event processing failed
			_, _ = client.ExitError(ctx, "ExtensionName.Reason", err)
			log.Panic(err)
		}
		if event.EventType == extapi.Shutdown {
			log.Println(event.ShutdownReason)
			os.Exit(0)
		}

		processEventFunc := func(event *extapi.NextEventResponse) error { return nil }
		if err := processEventFunc(event); err != nil {
			// 4. report error and exit if event processing failed
			_, _ = client.ExitError(ctx, "ExtensionName.Reason", err)
			log.Panic(err)
		}
	}
}

// Register supports optional arguments to override defaults.
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
		log.Panic(err)
	}
	_ = client
}

// Client.InitError accept error details to send to lambda.
func ExampleClient_InitError() {
	ctx := context.Background()

	client, err := extapi.Register(ctx)
	if err != nil {
		log.Panic(err)
	}

	// if something went wrong call client.InitError and exit without calling client.NextEvent.
	errResp, err := client.InitError(ctx, "ExtensionName.Reason", errors.New("text description of the error"))
	if err != nil {
		log.Println(err)
	}
	if errResp.Status != "OK" {
		log.Printf("unknown error response status: %s, want OK", errResp.Status)
	}
}

func ExampleClient_LogsSubscribe() {
	ctx := context.Background()
	destinationHostPort := "sandbox.localdomain:8080"

	// 1. start log receiving server
	srv := http.Server{
		Addr: destinationHostPort,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// process logs
		}),
	}
	defer func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	// 2. register extension and subscribe only to shutdown events
	client, err := extapi.Register(ctx, extapi.WithEventTypes([]extapi.EventType{extapi.Shutdown}))
	if err != nil {
		log.Panic(err)
	}

	// 3. subscribe to logs api
	req := extapi.NewLogsSubscribeRequest("http://"+destinationHostPort, nil, nil)
	if err := client.LogsSubscribe(ctx, req); err != nil {
		// 4. report error and exit if event processing failed
		_, _ = client.InitError(ctx, "ExtensionName.Reason", err)
		log.Panic(err)
	}

	// 5. block till shutdown event
	_, _ = client.NextEvent(ctx)
}

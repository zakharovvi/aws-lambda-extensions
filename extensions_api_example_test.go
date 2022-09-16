package lambdaextensions_test

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/zakharovvi/lambdaextensions"
)

// End to end example how to use Client, process events and handle errors
func ExampleClient() {
	ctx := context.Background()

	// 1. register extension
	client, err := lambdaextensions.Register(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println(client.RegisterResp.FunctionName)
	log.Println(client.RegisterResp.FunctionVersion)
	log.Println(client.RegisterResp.Handler)

	// 2. initialize the extension
	initFunc := func() error { return nil }
	if err := initFunc(); err != nil {
		// report error and exit if initialization failed
		_, _ = client.InitError(ctx, "ExtensionName.Reason", nil)
		log.Fatalln(err)
	}

	// 3. start polling events
	// first NextEvent calls notifies lambda that extension initialization has finished
	for {
		event, err := client.NextEvent(ctx)
		if err != nil {
			// report error and exit if event processing failed
			_, _ = client.ExitError(ctx, "ExtensionName.Reason", nil)
			log.Fatalln(err)
		}
		if event.EventType == lambdaextensions.Shutdown {
			log.Println(event.ShutdownReason)
			os.Exit(0)
		}

		processEventFunc := func(event *lambdaextensions.NextEventResponse) error { return nil }
		if err := processEventFunc(event); err != nil {
			// 4. report error and exit if event processing failed
			_, _ = client.ExitError(ctx, "ExtensionName.Reason", nil)
			log.Fatalln(err)
		}
	}
}

// Register supports optional arguments to override defaults
func ExampleRegister() {
	ctx := context.Background()

	client, err := lambdaextensions.Register(
		ctx,
		lambdaextensions.WithEventTypes([]lambdaextensions.EventType{lambdaextensions.Shutdown}),
		lambdaextensions.WithExtensionName("binary_file_basename"),
		lambdaextensions.WithAWSLambdaRuntimeAPI("127.0.0.1:8080"),
		lambdaextensions.WithHTTPClient(http.DefaultClient),
	)
	if err != nil {
		log.Fatalln(err)
	}
	_ = client
}

// Client.ExitError and Client.InitError accept error details to send to lambda
func ExampleClient_ExitError() {
	ctx := context.Background()

	client, err := lambdaextensions.Register(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	errorType := "Extension.UnknownReason"

	// ErrorRequest is optional
	errResp, err := client.ExitError(ctx, errorType, nil)
	if err != nil {
		log.Fatalln(err)
	}
	_ = errResp

	trace := strings.Split(string(debug.Stack()), "\n")
	errorReq := &lambdaextensions.ErrorRequest{
		ErrorMessage: "text description of the error",
		ErrorType:    errorType,
		StackTrace:   trace,
	}
	errResp, err = client.ExitError(ctx, errorType, errorReq)
	if err != nil {
		log.Fatalln(err)
	}
	_ = errResp
}
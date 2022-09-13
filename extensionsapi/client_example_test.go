package extensionsapi_test

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/zakharovvi/lambda-extension-api/extensionsapi"
)

func ExampleClient() {
	ctx := context.Background()

	client, err := extensionsapi.Register(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println(client.RegisterResp.FunctionName)
	log.Println(client.RegisterResp.FunctionVersion)
	log.Println(client.RegisterResp.Handler)

	event, err := client.NextEvent(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	if event.EventType == extensionsapi.Invoke {
		log.Println(event.RequestID)
	} else {
		log.Println(event.ShutdownReason)
		os.Exit(0)
	}
}

func ExampleRegister() {
	ctx := context.Background()

	client, err := extensionsapi.Register(
		ctx,
		extensionsapi.WithEventTypes([]extensionsapi.EventType{extensionsapi.Invoke}),
		extensionsapi.WithExtensionName("/path/to/binary"),
		extensionsapi.WithAWSLambdaRuntimeAPI("127.0.0.1:8080"),
		extensionsapi.WithHTTPClient(http.DefaultClient),
	)
	if err != nil {
		log.Fatalln(err)
	}
	_ = client
}

func ExampleError() {
	ctx := context.Background()

	client, err := extensionsapi.Register(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	errorType := "Extension.UnknownReason"

	// ErrorRequest is optional
	errResp, err := client.InitError(ctx, errorType, nil)
	if err != nil {
		log.Fatalln(err)
	}
	_ = errResp

	trace := strings.Split(string(debug.Stack()), "\n")
	errorReq := &extensionsapi.ErrorRequest{
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

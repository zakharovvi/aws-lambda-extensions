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

func ExampleClient() {
	ctx := context.Background()

	client, err := lambdaextensions.Register(ctx)
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

	if event.EventType == lambdaextensions.Invoke {
		log.Println(event.RequestID)
	} else {
		log.Println(event.ShutdownReason)
		os.Exit(0)
	}
}

func ExampleRegister() {
	ctx := context.Background()

	client, err := lambdaextensions.Register(
		ctx,
		lambdaextensions.WithEventTypes([]lambdaextensions.EventType{lambdaextensions.Invoke}),
		lambdaextensions.WithExtensionName("/path/to/binary"),
		lambdaextensions.WithAWSLambdaRuntimeAPI("127.0.0.1:8080"),
		lambdaextensions.WithHTTPClient(http.DefaultClient),
	)
	if err != nil {
		log.Fatalln(err)
	}
	_ = client
}

func ExampleError() {
	ctx := context.Background()

	client, err := lambdaextensions.Register(ctx)
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

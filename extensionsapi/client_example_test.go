package extensionsapi_test

import (
	"context"
	"github.com/zakharovvi/lambda-extension-framework/extensionsapi"
	"log"
	"net/http"
	"os"
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

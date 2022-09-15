package lambdaextensions_test

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/zakharovvi/lambdaextensions"
)

func ExampleClient_Subscribe() {
	ctx := context.Background()

	// 1. register extension and subscribe only to shutdown events
	client, err := lambdaextensions.Register(ctx, lambdaextensions.WithEventTypes([]lambdaextensions.EventType{lambdaextensions.Shutdown}))
	if err != nil {
		log.Fatalln(err)
	}

	// 2. start log receiving server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// process logs
	}))
	defer server.Close()

	// 3. subscribe to logs api
	req := lambdaextensions.NewSubscribeRequest(server.URL, nil)
	if err := client.Subscribe(ctx, req); err != nil {
		// 4. report error and exit if event processing failed
		_, _ = client.ExitError(ctx, "ExtensionName.Reason", nil)
		log.Fatalln(err)
	}

	// 5. wait for shutdown event
	for {
		_, _ = client.NextEvent(ctx)
	}
}

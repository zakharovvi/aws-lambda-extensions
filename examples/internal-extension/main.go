// Sample internal-extension demonstrates how to run an extension in the same binary with a lambda function.
package main

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
)

func HandleRequest(ctx context.Context) (string, error) {
	return "Hello world!", nil
}

type Extension struct {
	logger logr.Logger
	wg     sync.WaitGroup
}

func (ext *Extension) Init(ctx context.Context, client *extapi.Client) error {
	ext.logger.Info(
		"initializing extension...",
		"FunctionName", client.FunctionName(),
		"FunctionVersion", client.FunctionVersion(),
		"handler", client.Handler(),
		"extensionID", client.ExtensionID(),
	)
	ext.wg.Done()

	return nil
}

func (ext *Extension) HandleInvokeEvent(ctx context.Context, event *extapi.NextEventResponse) error {
	ctxDeadline, _ := ctx.Deadline()
	ext.logger.Info(
		"extension received invocation event",
		"requestId", event.RequestID,
		"invokedFunctionArn", event.InvokedFunctionArn,
		"timeout", ctxDeadline.Sub(time.Now()),
	)
	return nil
}

func (ext *Extension) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	ext.logger.Info(
		"shutting down extension...",
		"reason", reason,
		"error", err,
	)

	return nil
}

func (ext *Extension) Err() <-chan error {
	return nil
}

func main() {
	// log library debug messages
	stdr.SetVerbosity(1)

	// start the extension and wait for registration before proceeding to initialize the runtime
	ext := &Extension{
		logger: stdr.New(log.New(os.Stdout, "", log.Lshortfile)),
	}
	ext.wg.Add(1)
	go func() {
		if err := extapi.Run(
			context.Background(),
			ext,
			extapi.WithLogger(ext.logger),
			extapi.WithEventTypes([]extapi.EventType{extapi.Invoke}), // sam local invoke: ShutdownEventNotSupportedForInternalExtension
		); err != nil {
			log.Panic(err)
		}
	}()
	ext.wg.Wait()

	// run lambda runtime
	lambda.Start(HandleRequest)
}

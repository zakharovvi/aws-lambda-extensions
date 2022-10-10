package main

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
)

var journal *os.File

func HandleRequest(ctx context.Context) error {
	appendJournal("function.HandleRequest\n")
	return nil
}

type Extension struct {
	wg sync.WaitGroup
}

func (ext *Extension) Init(ctx context.Context, client *extapi.Client) error {
	appendJournal("extension.Init\n")
	ext.wg.Done()

	return nil
}

func (ext *Extension) HandleInvokeEvent(ctx context.Context, event *extapi.NextEventResponse) error {
	appendJournal("extension.HandleInvokeEvent\n")

	return nil
}

func (ext *Extension) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	// shutdown event type is not supported in AWS Lambda Runtime Interface Emulator

	return nil
}

func main() {
	var err error
	journal, err = os.Create("/tmp/rie-test-journal")
	if err != nil {
		log.Panic(err)
	}
	defer func() {
		journal.Close()
		os.Remove(journal.Name())
	}()

	// start the extension and wait for registration before proceeding to initialize the runtime
	ext := &Extension{}
	ext.wg.Add(1)
	go func() {
		if err := extapi.Run(
			context.Background(),
			ext,
			extapi.WithEventTypes([]extapi.EventType{extapi.Invoke}), // ShutdownEventNotSupportedForInternalExtension
		); err != nil {
			log.Panic(err)
		}
	}()
	ext.wg.Wait()

	// run lambda runtime
	lambda.Start(HandleRequest)
}

func appendJournal(s string) {
	if _, err := journal.WriteString(s); err != nil {
		log.Panic(err)
	}
	if err := journal.Sync(); err != nil {
		log.Panic(err)
	}
}

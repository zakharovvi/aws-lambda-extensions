package extapi_test

import (
	"context"
	"log"
	"net/http"

	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/logsapi"
)

type LogsExtension struct {
	srv    *http.Server
	logsCh chan logsapi.Log
}

func (ext *LogsExtension) Init(ctx context.Context, client *extapi.Client) error {
	// 1. start log processing
	go func() {
		for msg := range ext.logsCh {
			log.Printf("time=%s type=%s\n", msg.LogType, msg.Time)
		}
	}()

	// 2. start http server
	go func() {
		if err := ext.srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Println(err)
		}
	}()

	// 3. subscribe to lambda logs
	req := extapi.NewLogsSubscribeRequest(ext.srv.Addr, nil)

	return client.LogsSubscribe(ctx, req)
}

func (ext *LogsExtension) HandleInvokeEvent(ctx context.Context, event *extapi.NextEventResponse) error {
	panic("for log subscriber extension example we don't subscribe to 'invoke' events. 'shutdown' event will be handled by run")
}

func (ext *LogsExtension) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	log.Printf("shutting down extension due to reason=%s error=%s\n", reason, err)
	// gracefully shut down logs receiver http server
	err = ext.srv.Shutdown(ctx)
	close(ext.logsCh)

	return err
}

func Example_logsSubscribe() {
	// 1. init http server
	logsCh := make(chan logsapi.Log)
	srv := &http.Server{
		Addr: ":0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := logsapi.DecodeLogs(r.Context(), r.Body, logsCh); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				log.Println(err)

				return
			}
			w.WriteHeader(http.StatusOK)
		}),
	}

	// 2. run extension
	ext := &LogsExtension{srv, logsCh}
	if err := extapi.Run(
		context.Background(),
		ext,
		extapi.WithEventTypes([]extapi.EventType{extapi.Shutdown}), // subscribe only to shutdown events
	); err != nil {
		log.Panicln(err)
	}
}

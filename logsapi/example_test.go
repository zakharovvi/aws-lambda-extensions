package logsapi_test

import (
	"log"
	"net/http"

	"github.com/zakharovvi/aws-lambda-extensions/logsapi"
)

func ExampleDecodeLogs() {
	// 1. create channel for decoded logs
	logsCh := make(chan logsapi.Log)

	// 2. consume decoded logs from channel
	go func() {
		for msg := range logsCh {
			log.Println(msg.LogType)
			log.Println(msg.Time)

			// 3. type cast log records and access fields
			report, ok := msg.Record.(logsapi.RecordPlatformReport)
			if !ok {
				continue
			}
			log.Println(report.RequestID)
			log.Println(report.Metrics.BilledDurationMs)
			log.Println(report.Metrics.MaxMemoryUsedMB)
		}
	}()

	// 4. use DecodeLogs in HTTP handler
	http.HandleFunc("/logs-receiver", func(w http.ResponseWriter, r *http.Request) {
		if err := logsapi.DecodeLogs(r.Body, logsCh); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Println(err)

			return
		}
		w.WriteHeader(http.StatusOK)
	})
	log.Panic(http.ListenAndServe("", nil))
}

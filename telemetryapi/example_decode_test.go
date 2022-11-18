package telemetryapi_test

import (
	"log"
	"net/http"

	"github.com/zakharovvi/aws-lambda-extensions/telemetryapi"
)

func ExampleDecode() {
	// 1. create channel for decoded events
	eventsCh := make(chan telemetryapi.Event)

	// 2. consume decoded events from the channel
	go func() {
		for msg := range eventsCh {
			log.Println(msg.Type)
			log.Println(msg.Time)

			// 3. type cast log records and access fields
			report, ok := msg.Record.(telemetryapi.RecordPlatformReport)
			if !ok {
				continue
			}
			log.Println(report.RequestID)
			log.Println(report.Metrics.BilledDuration)
			log.Println(report.Metrics.MaxMemoryUsedMB)
		}
	}()

	// 4. use Decode in HTTP handler
	http.HandleFunc("/telemetry-receiver", func(w http.ResponseWriter, r *http.Request) {
		if err := telemetryapi.Decode(r.Context(), r.Body, eventsCh); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Println(err)

			return
		}
	})
	log.Panic(http.ListenAndServe("", nil))
}

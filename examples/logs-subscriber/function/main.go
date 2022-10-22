// Sample lambda function to demonstrate Logs API extension.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
)

func HandleRequest(ctx context.Context) (string, error) {
	startedAt := time.Now()
	for i := 0; i < 10; i++ {
		log.Printf("function is working for %v", time.Since(startedAt))
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Sprintf("function stopped after %v", time.Since(startedAt)), nil
}

func main() {
	lambda.Start(HandleRequest)
}

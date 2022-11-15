package lambdaext

import (
	"encoding/json"
	"time"
)

type AWSLambdaRuntimeAPI string

type RequestID string

type ExtensionName string

type FunctionVersion string

// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#InitType
type InitType string

const (
	InitTypeOnDemand               InitType = "on-demand"
	InitTypeProvisionedConcurrency InitType = "provisioned-concurrency"
)

// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#TracingType
type TracingType string

const TracingTypeAWSXRay TracingType = "X-Amzn-Trace-Id"

type TracingValue string

type DurationMs time.Duration

func (d *DurationMs) UnmarshalJSON(b []byte) error {
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*d = DurationMs(v * float64(time.Millisecond))

	return nil
}

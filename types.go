package lambdaext

import (
	"encoding/json"
	"fmt"
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
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch val := v.(type) {
	case float64:
		*d = DurationMs(val * float64(time.Millisecond))
	case int:
		*d = DurationMs(val * int(time.Millisecond))
	default:
		return fmt.Errorf("invalid duration: %#v", v)
	}

	return nil
}

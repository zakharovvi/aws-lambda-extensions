package lambdaext

import (
	"encoding/json"
	"fmt"
	"time"
)

// AWSLambdaRuntimeAPI is the API endpoint retrieved from the AWS_LAMBDA_RUNTIME_API environment variable from execution environment.
type AWSLambdaRuntimeAPI string

type RequestID string

// ExtensionName is the full file name of the extension.
type ExtensionName string

// FunctionVersion is created a new version of your function each time that you publish the function.
// https://docs.aws.amazon.com/lambda/latest/dg/configuration-versions.html
type FunctionVersion string

// InitType describes how Lambda initialized the environment.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#InitType
type InitType string

const (
	InitTypeOnDemand               InitType = "on-demand"
	InitTypeProvisionedConcurrency InitType = "provisioned-concurrency"
)

// TracingType describes the type of tracing in a TraceContext object.
// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#TracingType
type TracingType string

const TracingTypeAWSXRay TracingType = "X-Amzn-Trace-Id"

type TracingValue string

// DurationMs is a time.Duration, parsed from numeric milliseconds value.
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

func (d DurationMs) String() string {
	return time.Duration(d).String()
}

func (d DurationMs) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, d)), nil
}

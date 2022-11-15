package lambdaext

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

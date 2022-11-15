package lambdaext

type RequestID string

type ExtensionName string

type FunctionVersion string

// https://docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#InitType
type InitType string

const (
	InitTypeOnDemand               InitType = "on-demand"
	InitTypeProvisionedConcurrency InitType = "provisioned-concurrency"
)

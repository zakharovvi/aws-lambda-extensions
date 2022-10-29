package extapi

import (
	"os"
	"strconv"
)

// Defined runtime environment variables.
// Lambda runtimes set several environment variables during initialization.
// Most of the environment variables provide information about the function or runtime.
// The keys for these environment variables are reserved and cannot be set in your function configuration.
// https://docs.aws.amazon.com/lambda/latest/dg/configuration-envvars.html#configuration-envvars-runtime
// https://docs.aws.amazon.com/lambda/latest/dg/runtimes-extensions-api.html#runtimes-extensions-registration-api-e

// EnvXAmznTraceID returns X-Ray tracing header.
func EnvXAmznTraceID() string {
	return os.Getenv("_X_AMZN_TRACE_ID")
}

// EnvAWSRegion returns the AWS Region where the Lambda function is executed.
func EnvAWSRegion() string {
	return os.Getenv("AWS_REGION")
}

// EnvAWSLambdaFunctionName returns the name of the function.
func EnvAWSLambdaFunctionName() string {
	return os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
}

// EnvAWSLambdaFunctionMemorySizeMB returns the amount of memory available to the function in MB.
func EnvAWSLambdaFunctionMemorySizeMB() int {
	s := os.Getenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE")
	n, _ := strconv.Atoi(s)

	return n
}

// EnvAWSLambdaFunctionVersion returns the version of the function being executed.
func EnvAWSLambdaFunctionVersion() string {
	return os.Getenv("AWS_LAMBDA_FUNCTION_VERSION")
}

// EnvAWSLambdaInitializationType returns the initialization type of the function, which is either on-demand or provisioned-concurrency. For information, see Configuring provisioned concurrency.
func EnvAWSLambdaInitializationType() string {
	return os.Getenv("AWS_LAMBDA_INITIALIZATION_TYPE")
}

// EnvAWSLambdaRuntimeAPI returns the host and port of the runtime API for custom runtime.
func EnvAWSLambdaRuntimeAPI() string {
	return os.Getenv("AWS_LAMBDA_RUNTIME_API")
}

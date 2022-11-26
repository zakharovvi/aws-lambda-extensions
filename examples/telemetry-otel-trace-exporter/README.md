# Example Telemetry API Extension

This example demonstrates how to convert Telemetry API events into OpenTelemetry tracing spans.

[Converting Lambda Telemetry API Event objects to OpenTelemetry Spans](https://docs.aws.amazon.com/lambda/latest/dg/telemetry-otel-spans.html)

## Usage

### Prerequisites

* [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install-mac.html)
  installed
* `go` installed
* AWS credentials configured

### Steps

1. build extension `GOWORK=/Users/zakharovvi/go/src/github.com/zakharovvi/aws-lambda-extensions/go.work sam build`
1. validate SAM template: `sam validate`
1. test Function in the Cloud: `sam sync --stack-name {stack-name} --watch`

Telemetry API is not supported in `sam local invoke`.

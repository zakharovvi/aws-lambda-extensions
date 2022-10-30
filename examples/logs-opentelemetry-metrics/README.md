# Example Logs API to OpenTelemetry Metric Extension

This example demonstrates how to use Lambda Logs API, parse record fields, and convert them into OpenTelemetry metrics.

## Usage

### Prerequisites

* [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install-mac.html)
  installed
* `go` installed
* AWS credentials configured

### Steps

1. build extension `sam build`
1. validate SAM template: `sam validate`
1. test Function in the Cloud: `sam sync --stack-name {stack-name} --watch`

Logs API is not supported in `sam local invoke`.

# Example Logs API to OpenTelemetry Metric Extension

> **Warning**
> The Lambda Telemetry API supersedes the Lambda Logs API.
> While the Logs API remains fully functional, we recommend using only the Telemetry API going forward.
> You can subscribe your extension to a telemetry stream using either the Telemetry API or the Logs API.
> After subscribing using one of these APIs, any attempt to subscribe using the other API returns an error.
> * [Introducing the AWS Lambda Telemetry API](https://docs.aws.amazon.com/lambda/latest/dg/runtimes-logs-api.html)
> * [Lambda Logs API](https://aws.amazon.com/blogs/compute/introducing-the-aws-lambda-telemetry-api/)

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

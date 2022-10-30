# Example Logs API Extension

This example demonstrates how to use Lambda Logs API and how to deploy extension as a separate binary in a lambda layer.

## Usage

### Prerequisites

* [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install-mac.html) installed
* `go` installed
* AWS credentials configured

### Steps

1. build extension `GOWORK=/Users/zakharovvi/go/src/github.com/zakharovvi/aws-lambda-extensions/go.work sam build`
1. validate SAM template: `sam validate`
1. test Function in the Cloud: `sam sync --stack-name {stack-name} --watch`

Logs API is not supported in `sam local invoke`.

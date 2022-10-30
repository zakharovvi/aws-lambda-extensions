# Example Internal Extension

This example demonstrates how to use the library to build extension embedded into function binary.

## Usage

### Prerequisites

* [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install-mac.html) installed
* `go` installed
* `docker` installed
* AWS credentials configured

### Steps

1. build extension `sam build`
1. validate SAM template: `sam validate`
1. run function locally: `sam local invoke`

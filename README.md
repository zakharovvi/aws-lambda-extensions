# AWS Lambda Extensions library

[![Go Reference](https://pkg.go.dev/badge/github.com/zakharovvi/aws-lambda-extensions.svg)](https://pkg.go.dev/github.com/zakharovvi/aws-lambda-extensions)
[![ci](https://github.com/zakharovvi/aws-lambda-extensions/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/zakharovvi/aws-lambda-extensions/actions/workflows/ci.yml)

This repository contains framework and helper functions to build your own AWS lambda extensions in Go.

## Overview

Repository contains two main packages:
* [extapi](https://pkg.go.dev/github.com/zakharovvi/aws-lambda-extensions/extapi) for [Extensions API](https://docs.aws.amazon.com/lambda/latest/dg/runtimes-extensions-api.html)
* [logsapi](https://pkg.go.dev/github.com/zakharovvi/aws-lambda-extensions/logsapi) for [Logs API](https://docs.aws.amazon.com/lambda/latest/dg/runtimes-logs-api.html)

You can find more information on how to build your lambda extensions in [AWS documentation](https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtime-environment.html).

## Examples

* [example extensions](examples)
* [examples in go reference](https://pkg.go.dev/github.com/zakharovvi/aws-lambda-extensions)

module github.com/zakharovvi/aws-lambda-extensions/examples/internal-extension

go 1.18

replace github.com/zakharovvi/lambda-extensions => ../../

require (
	github.com/aws/aws-lambda-go v1.34.1
	github.com/go-logr/logr v1.2.3
	github.com/go-logr/stdr v1.2.2
	github.com/zakharovvi/lambda-extensions v0.0.0-00010101000000-000000000000
)

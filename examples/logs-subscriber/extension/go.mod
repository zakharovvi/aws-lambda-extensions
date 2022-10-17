module github.com/zakharovvi/aws-lambda-extensions/examples/logs-subscriber/extension

go 1.18

// can't use relative path as "sam build" copies sources to temp directory before calling "go build"
replace github.com/zakharovvi/aws-lambda-extensions => /Users/zakharovvi/go/src/github.com/zakharovvi/aws-lambda-extensions

require (
	github.com/go-logr/stdr v1.2.2
	github.com/zakharovvi/aws-lambda-extensions v0.0.0-00010101000000-000000000000
)

require github.com/go-logr/logr v1.2.3 // indirect

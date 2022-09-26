module github.com/zakharovvi/aws-lambda-extensions/tests/rie

go 1.18

replace github.com/zakharovvi/aws-lambda-extensions => ../../

require (
	github.com/aws/aws-lambda-go v1.34.1
	github.com/zakharovvi/aws-lambda-extensions v0.0.0-00010101000000-000000000000
)

require github.com/go-logr/logr v1.2.3 // indirect

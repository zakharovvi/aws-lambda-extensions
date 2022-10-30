export GOWORK := $(shell pwd)/../../go.work

build:
	sam build

validate:
	aws-vault exec default -- sam validate

sync:
	aws-vault exec default -- sam sync --stack-name logs-subscriber

watch:
	aws-vault exec default -- sam sync --stack-name logs-subscriber --watch

.PHONY: build validate sync watch

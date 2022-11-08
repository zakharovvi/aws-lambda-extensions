export GOWORK := $(shell pwd)/../../go.work

build:
	sam build

validate:
	aws-vault exec default -- sam validate

sync:
	aws-vault exec default -- sam sync

watch:
	aws-vault exec default -- sam sync --watch

.PHONY: build validate sync watch

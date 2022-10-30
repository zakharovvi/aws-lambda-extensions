build:
	sam build

validate:
	aws-vault exec default -- sam validate

local-invoke:
	sam local invoke

watch:
	aws-vault exec default -- sam sync --stack-name example-internal-extension --watch

.PHONY: build validate local-invoke watch

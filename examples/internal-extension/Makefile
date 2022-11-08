build:
	sam build

validate:
	aws-vault exec default -- sam validate

local-invoke:
	sam local invoke

sync:
	aws-vault exec default -- sam sync

watch:
	aws-vault exec default -- sam sync --watch

.PHONY: build validate local-invoke sync watch

.PHONY: up down generate test-go test-py test-web test-all hooks ui-mock

up:
	docker compose -f infrastructure/local/compose.yaml up -d --wait

down:
	docker compose -f infrastructure/local/compose.yaml down -v

generate:
	buf generate contracts
	buf breaking contracts --against '.git#branch=main'

test-go:
	go test $$(go list -m -f '{{if .Main}}{{.Path}}/...{{end}}' all)

test-py:
	@find tools services -name pyproject.toml -type f -print | sort | while read -r project; do \
		dir=$${project%/*}; \
		uv run --project "$$dir" pytest "$$dir"; \
	done

test-web:
	pnpm --dir apps/console test

test-all: test-go test-py test-web

hooks:
	git config core.hooksPath tools/development/hooks

ui-mock:
	go run ./tools/development/mockapi

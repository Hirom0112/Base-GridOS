.PHONY: up down generate test-go test-py test-web test-all hooks ui-mock

up:
	docker compose -f infrastructure/local/compose.yaml up -d --wait

down:
	docker compose -f infrastructure/local/compose.yaml down -v

generate:
	buf generate contracts
	buf breaking contracts --against '.git#branch=main'

test-go:
	go test ./...

test-py:
	uv run pytest

test-web:
	pnpm --dir apps/console test

test-all: test-go test-py test-web

hooks:
	git config core.hooksPath tools/development/hooks

ui-mock:
	go run ./tools/development/mockapi

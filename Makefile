.PHONY: up down generate test-go test-py test-web test-all hooks ui-mock plugins decision

up:
	docker compose -f infrastructure/local/compose.yaml up -d --wait

down:
	docker compose -f infrastructure/local/compose.yaml down -v

generate:
	buf generate contracts
	buf breaking contracts --against '.git#branch=main,subdir=contracts'

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

plugins:
	mkdir -p .local/bin
	GOBIN="$(CURDIR)/.local/bin" go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	GOBIN="$(CURDIR)/.local/bin" go install connectrpc.com/connect/cmd/protoc-gen-connect-go@v1.21.0
	pnpm --package=@bufbuild/protoc-gen-es@2.11.0 dlx protoc-gen-es --version
	uv sync --project services/decision
	printf '%s\n' '#!/bin/sh' 'exec uv run --offline --project services/decision python -m grpc_tools.protoc "$$@"' > .local/bin/protoc
	chmod +x .local/bin/protoc

decision:
	uv run --project services/decision python -m gridos.server --port "$${GRIDOS_DECISION_PORT:-50061}"

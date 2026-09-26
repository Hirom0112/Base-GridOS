.PHONY: up down generate test-go test-e2e test-py test-web test-all hooks ui-mock plugins decision demo

up:
	docker compose -f infrastructure/local/compose.yaml up -d --wait

down:
	docker compose -f infrastructure/local/compose.yaml down -v

generate:
	buf generate contracts
	buf breaking contracts --against '.git#branch=main,subdir=contracts'

test-go:
	go test $$(go list -m -f '{{if and .Main (ne .Path "github.com/Hirom0112/Base-GridOS/tests/end-to-end")}}{{.Path}}/...{{end}}' all)

test-e2e:
	@set -e; \
		mkdir -p .local/demo; \
		$(MAKE) demo > .local/demo/test-e2e.log 2>&1 & demo_pid=$$!; \
		cleanup() { kill $$demo_pid 2>/dev/null || true; wait $$demo_pid 2>/dev/null || true; $(MAKE) down; }; \
		trap cleanup EXIT INT TERM; \
		attempts=0; until grep -q '^demo ready:' .local/demo/test-e2e.log; do kill -0 $$demo_pid; attempts=$$((attempts + 1)); test $$attempts -lt 120 || { tail -n 20 .local/demo/test-e2e.log; exit 1; }; sleep 0.25; done; \
		go test ./tests/end-to-end/

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

demo: up
	go run ./services/control/cmd/migrate up
	psql "$${GRIDOS_DATABASE_URL:-postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable}" -v ON_ERROR_STOP=1 -f database/seeds/dev.sql
	mkdir -p .local/demo
	@if test -f .local/demo/pids; then while read -r pid; do kill "$$pid" 2>/dev/null || true; done < .local/demo/pids; fi
	go build -o .local/demo/gateway ./services/gateway-simulator/cmd/gateway-simulator
	go build -o .local/demo/control ./services/control/cmd/control
	@set -e; \
		: > .local/demo/pids; \
		env GRIDOS_GATEWAY_TOKEN='Bearer local-gateway' .local/demo/gateway --address :28081 --database .local/demo/gateway.db --fleet testdata/fleets/austin-5000.jsonl --gateway-id demo-gateway --scenario-start "$$(date -u +%Y-%m-%dT%H:%M:%SZ)" > .local/demo/gateway.log 2>&1 & echo $$! >> .local/demo/pids; \
		uv run --project services/decision python -m gridos.server --port 25061 > .local/demo/decision.log 2>&1 & echo $$! >> .local/demo/pids; \
		env GRIDOS_CONTROL_ADDRESS=:28080 GRIDOS_GATEWAY_TOKEN='Bearer local-gateway' GRIDOS_GATEWAY_ADDR=http://localhost:28081 GRIDOS_DECISION_ADDR=http://localhost:25061 GRIDOS_FLEET=testdata/fleets/austin-5000.jsonl .local/demo/control > .local/demo/control.log 2>&1 & echo $$! >> .local/demo/pids; \
		if test -f apps/console/package.json; then pnpm --dir apps/console dev > .local/demo/console.log 2>&1 & echo $$! >> .local/demo/pids; fi; \
		trap 'while read -r pid; do kill "$$pid" 2>/dev/null || true; done < .local/demo/pids' INT TERM EXIT; \
		for port in 28081 25061 28080; do attempts=0; until nc -z 127.0.0.1 $$port; do attempts=$$((attempts + 1)); test $$attempts -lt 120 || { tail -n 20 .local/demo/*.log; exit 1; }; sleep 0.25; done; done; \
		echo 'demo ready: control=:28080 decision=:25061 gateway=:28081'; \
		wait

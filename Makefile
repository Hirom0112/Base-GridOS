.PHONY: up down generate test-go test-e2e test-py test-web test-all hooks ui-mock plugins decision demo

DEMO_CONTROL_PORT ?= 28080
DEMO_DECISION_PORT ?= 25061
DEMO_GATEWAY_PORT ?= 28081
DEMO_DIR ?= .local/demo

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
		mkdir -p .local/e2e; \
		$(MAKE) demo DEMO_DIR=.local/e2e DEMO_CONTROL_PORT=38080 DEMO_DECISION_PORT=35061 DEMO_GATEWAY_PORT=38081 > .local/e2e/test-e2e.log 2>&1 & demo_pid=$$!; \
		cleanup() { kill $$demo_pid 2>/dev/null || true; wait $$demo_pid 2>/dev/null || true; }; \
		trap cleanup EXIT INT TERM; \
		attempts=0; until grep -q '^demo ready:' .local/e2e/test-e2e.log; do kill -0 $$demo_pid; attempts=$$((attempts + 1)); test $$attempts -lt 120 || { tail -n 20 .local/e2e/test-e2e.log; exit 1; }; sleep 0.25; done; \
		env GRIDOS_CONTROL_URL=http://localhost:38080 GRIDOS_GATEWAY_URL=http://localhost:38081 GRIDOS_GATEWAY_DATABASE=$(CURDIR)/.local/e2e/gateway.db go test ./tests/end-to-end/

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
	mkdir -p $(DEMO_DIR)
	@if test -f $(DEMO_DIR)/pids; then while read -r pid; do kill "$$pid" 2>/dev/null || true; done < $(DEMO_DIR)/pids; fi
	go build -o $(DEMO_DIR)/gateway ./services/gateway-simulator/cmd/gateway-simulator
	go build -o $(DEMO_DIR)/control ./services/control/cmd/control
	@set -e; \
		: > $(DEMO_DIR)/pids; \
		env GRIDOS_GATEWAY_TOKEN='Bearer local-gateway' $(DEMO_DIR)/gateway --address :$(DEMO_GATEWAY_PORT) --database $(DEMO_DIR)/gateway.db --fleet testdata/fleets/austin-5000.jsonl --gateway-id demo-gateway --scenario-start "$$(date -u +%Y-%m-%dT%H:%M:%SZ)" > $(DEMO_DIR)/gateway.log 2>&1 & echo $$! >> $(DEMO_DIR)/pids; \
		uv run --project services/decision python -m gridos.server --port $(DEMO_DECISION_PORT) > $(DEMO_DIR)/decision.log 2>&1 & echo $$! >> $(DEMO_DIR)/pids; \
		env GRIDOS_CONTROL_ADDRESS=:$(DEMO_CONTROL_PORT) GRIDOS_GATEWAY_TOKEN='Bearer local-gateway' GRIDOS_GATEWAY_ADDR=http://localhost:$(DEMO_GATEWAY_PORT) GRIDOS_DECISION_ADDR=http://localhost:$(DEMO_DECISION_PORT) GRIDOS_FLEET=testdata/fleets/austin-5000.jsonl $(DEMO_DIR)/control > $(DEMO_DIR)/control.log 2>&1 & echo $$! >> $(DEMO_DIR)/pids; \
		if test -f apps/console/package.json && test "$(DEMO_DIR)" = ".local/demo"; then pnpm --dir apps/console dev > $(DEMO_DIR)/console.log 2>&1 & echo $$! >> $(DEMO_DIR)/pids; fi; \
		trap 'while read -r pid; do kill "$$pid" 2>/dev/null || true; done < $(DEMO_DIR)/pids' INT TERM EXIT; \
		for port in $(DEMO_GATEWAY_PORT) $(DEMO_DECISION_PORT) $(DEMO_CONTROL_PORT); do attempts=0; until nc -z 127.0.0.1 $$port; do attempts=$$((attempts + 1)); test $$attempts -lt 120 || { tail -n 20 $(DEMO_DIR)/*.log; exit 1; }; sleep 0.25; done; done; \
		echo 'demo ready: control=:$(DEMO_CONTROL_PORT) decision=:$(DEMO_DECISION_PORT) gateway=:$(DEMO_GATEWAY_PORT)'; \
		wait

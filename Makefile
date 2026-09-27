.PHONY: up down generate test-go test-e2e test-py test-web test-all hooks ui-mock plugins decision demo

GRIDOS_DEMO_CONTROL_PORT ?= 28080
GRIDOS_DEMO_DECISION_PORT ?= 25061
GRIDOS_DEMO_GATEWAY_PORT ?= 28081
GRIDOS_DEMO_DIR ?= .local/demo
GRIDOS_DEMO_TASK_QUEUE ?= gridos-dispatch
DEMO_DATABASE_URL ?= postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable

up:
	docker compose -f infrastructure/local/compose.yaml up -d --wait

down:
	docker compose -f infrastructure/local/compose.yaml down -v

generate:
	buf generate contracts
	buf breaking contracts --against '.git#branch=main,subdir=contracts'

test-go:
	go test -timeout 40m $$(go list -m -f '{{if and .Main (ne .Path "github.com/Hirom0112/Base-GridOS/tests/end-to-end")}}{{.Path}}/...{{end}}' all)

test-e2e:
	@set -e; \
		database_url='postgres://gridos:gridos@localhost:5432/gridos_e2e?sslmode=disable'; \
		admin_url='postgres://gridos:gridos@localhost:5432/postgres?sslmode=disable'; \
		mkdir -p .local/e2e; \
		rm -f .local/e2e/gateway.db; \
		psql "$$admin_url" -v ON_ERROR_STOP=1 -c 'DROP DATABASE IF EXISTS gridos_e2e WITH (FORCE)' >/dev/null; \
		psql "$$admin_url" -v ON_ERROR_STOP=1 -c 'CREATE DATABASE gridos_e2e' >/dev/null; \
		GRIDOS_CONTROL_METRICS_ADDRESS=127.0.0.1:0 GRIDOS_WORKER_METRICS_ADDRESS=127.0.0.1:0 GRIDOS_GATEWAY_METRICS_ADDRESS=127.0.0.1:0 GRIDOS_DECISION_METRICS_ADDRESS=127.0.0.1:0 $(MAKE) demo DEMO_DATABASE_URL="$$database_url" GRIDOS_DEMO_DIR=.local/e2e GRIDOS_DEMO_TASK_QUEUE=gridos-e2e GRIDOS_DEMO_CONTROL_PORT=38080 GRIDOS_DEMO_DECISION_PORT=35061 GRIDOS_DEMO_GATEWAY_PORT=38081 > .local/e2e/test-e2e.log 2>&1 & demo_pid=$$!; \
		cleanup() { if test -f .local/e2e/pids; then while read -r pid; do kill "$$pid" 2>/dev/null || true; done < .local/e2e/pids; fi; kill $$demo_pid 2>/dev/null || true; wait $$demo_pid 2>/dev/null || true; psql "$$admin_url" -v ON_ERROR_STOP=1 -c 'DROP DATABASE IF EXISTS gridos_e2e WITH (FORCE)' >/dev/null; }; \
		trap cleanup EXIT INT TERM; \
		attempts=0; until grep -q '^demo ready:' .local/e2e/test-e2e.log; do kill -0 $$demo_pid; attempts=$$((attempts + 1)); test $$attempts -lt 120 || { tail -n 20 .local/e2e/test-e2e.log; exit 1; }; sleep 0.25; done; \
		env GRIDOS_DATABASE_URL="$$database_url" GRIDOS_CONTROL_URL=http://localhost:38080 GRIDOS_GATEWAY_URL=http://localhost:38081 GRIDOS_GATEWAY_DATABASE=$(CURDIR)/.local/e2e/gateway.db go test -count=1 ./tests/end-to-end/

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
	env GRIDOS_DATABASE_URL='$(DEMO_DATABASE_URL)' go run ./services/control/cmd/migrate up
	psql '$(DEMO_DATABASE_URL)' -v ON_ERROR_STOP=1 -f database/seeds/dev.sql
	mkdir -p $(GRIDOS_DEMO_DIR)
	@if test -f $(GRIDOS_DEMO_DIR)/pids; then while read -r pid; do kill "$$pid" 2>/dev/null || true; done < $(GRIDOS_DEMO_DIR)/pids; fi
	go build -o $(GRIDOS_DEMO_DIR)/gateway ./services/gateway-simulator/cmd/gateway-simulator
	go build -o $(GRIDOS_DEMO_DIR)/control ./services/control/cmd/control
	go build -o $(GRIDOS_DEMO_DIR)/worker ./services/control/cmd/worker
	@set -e; \
		: > $(GRIDOS_DEMO_DIR)/pids; \
		env GRIDOS_GATEWAY_METRICS_ADDRESS=$${GRIDOS_GATEWAY_METRICS_ADDRESS:-0.0.0.0:9466} GRIDOS_GATEWAY_TOKEN='Bearer local-gateway' $(GRIDOS_DEMO_DIR)/gateway --address :$(GRIDOS_DEMO_GATEWAY_PORT) --control-address http://localhost:$(GRIDOS_DEMO_CONTROL_PORT) --database $(GRIDOS_DEMO_DIR)/gateway.db --fleet testdata/fleets/austin-5000.jsonl --gateway-id demo-gateway --cadence 15s --scenario-start "$$(date -u +%Y-%m-%dT%H:%M:%SZ)" > $(GRIDOS_DEMO_DIR)/gateway.log 2>&1 & echo $$! >> $(GRIDOS_DEMO_DIR)/pids; \
		env GRIDOS_DECISION_METRICS_ADDRESS=$${GRIDOS_DECISION_METRICS_ADDRESS:-0.0.0.0:9467} uv run --project services/decision python -m gridos.server --port $(GRIDOS_DEMO_DECISION_PORT) > $(GRIDOS_DEMO_DIR)/decision.log 2>&1 & echo $$! >> $(GRIDOS_DEMO_DIR)/pids; \
		env GRIDOS_CONTROL_METRICS_ADDRESS=$${GRIDOS_CONTROL_METRICS_ADDRESS:-0.0.0.0:9464} GRIDOS_DATABASE_URL='$(DEMO_DATABASE_URL)' GRIDOS_CONTROL_ADDRESS=:$(GRIDOS_DEMO_CONTROL_PORT) GRIDOS_GATEWAY_TOKEN='Bearer local-gateway' GRIDOS_GATEWAY_ADDR=http://localhost:$(GRIDOS_DEMO_GATEWAY_PORT) GRIDOS_DECISION_ADDR=http://localhost:$(GRIDOS_DEMO_DECISION_PORT) GRIDOS_FLEET=testdata/fleets/austin-5000.jsonl GRIDOS_TASK_QUEUE=$(GRIDOS_DEMO_TASK_QUEUE) $(GRIDOS_DEMO_DIR)/control > $(GRIDOS_DEMO_DIR)/control.log 2>&1 & echo $$! >> $(GRIDOS_DEMO_DIR)/pids; \
		env GRIDOS_WORKER_METRICS_ADDRESS=$${GRIDOS_WORKER_METRICS_ADDRESS:-0.0.0.0:9465} GRIDOS_DATABASE_URL='$(DEMO_DATABASE_URL)' GRIDOS_GATEWAY_TOKEN='Bearer local-gateway' GRIDOS_GATEWAY_ADDR=http://localhost:$(GRIDOS_DEMO_GATEWAY_PORT) GRIDOS_DECISION_ADDR=http://localhost:$(GRIDOS_DEMO_DECISION_PORT) GRIDOS_FLEET=testdata/fleets/austin-5000.jsonl GRIDOS_TASK_QUEUE=$(GRIDOS_DEMO_TASK_QUEUE) $(GRIDOS_DEMO_DIR)/worker > $(GRIDOS_DEMO_DIR)/worker.log 2>&1 & worker_pid=$$!; echo $$worker_pid >> $(GRIDOS_DEMO_DIR)/pids; \
		if test -f apps/console/package.json && test "$(GRIDOS_DEMO_DIR)" = ".local/demo"; then pnpm --dir apps/console dev > $(GRIDOS_DEMO_DIR)/console.log 2>&1 & echo $$! >> $(GRIDOS_DEMO_DIR)/pids; fi; \
		trap 'while read -r pid; do kill "$$pid" 2>/dev/null || true; done < $(GRIDOS_DEMO_DIR)/pids' INT TERM EXIT; \
		for port in $(GRIDOS_DEMO_GATEWAY_PORT) $(GRIDOS_DEMO_DECISION_PORT) $(GRIDOS_DEMO_CONTROL_PORT); do attempts=0; until nc -z 127.0.0.1 $$port; do attempts=$$((attempts + 1)); test $$attempts -lt 120 || { tail -n 20 $(GRIDOS_DEMO_DIR)/*.log; exit 1; }; sleep 0.25; done; done; \
		attempts=0; until grep -q 'Started Worker' $(GRIDOS_DEMO_DIR)/worker.log && kill -0 $$worker_pid 2>/dev/null; do attempts=$$((attempts + 1)); test $$attempts -lt 120 || { tail -n 20 $(GRIDOS_DEMO_DIR)/worker.log; exit 1; }; sleep 0.25; done; \
		echo 'demo ready: control=:$(GRIDOS_DEMO_CONTROL_PORT) decision=:$(GRIDOS_DEMO_DECISION_PORT) gateway=:$(GRIDOS_DEMO_GATEWAY_PORT)'; \
		wait

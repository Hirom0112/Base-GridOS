# GridOS

GridOS coordinates a distributed fleet of residential batteries as dependable
grid capacity while protecting each household's backup reserve. It forecasts
conditions, builds and validates safe dispatch plans, executes them through a
failure-tolerant workflow, and verifies the power actually delivered.
Members can choose a resilience plan or schedule Travel Flex; additional
capacity is used only when safety protections hold and conservative incremental
margin remains positive.

## Run locally

Install Go, `uv`, `pnpm`, Docker Compose, the PostgreSQL client, `buf`, and
`sqlc` as listed in [`AGENTS.md`](AGENTS.md). From a clean checkout, keep
ports 5432, 7233, 3000, 25061, 28080–28081, and 9464–9467
free, then run:

```sh
make plugins
make generate
pnpm --dir apps/console install --frozen-lockfile
make up
GRIDOS_DEMO_SCENARIO=testdata/scenarios/heat-event-canonical.yaml make demo
```

Open [the console](http://127.0.0.1:3000) after `demo ready` appears. The demo
uses a simulated Austin fleet; follow the [17-step operator
walkthrough](docs/operations/demo.md). Run `make test-go` for the Go test suite.

## Current status

A runnable local MVP includes the control API and worker, decision service,
gateway simulator, console, durable storage, and a live scenario demo. All 17
Wave 4 backend scenarios passed. Wave 5 acceptance and the complete browser
demo path are still in progress; this is not a production deployment.

## Start here

- [`FULL_SPEC.md`](FULL_SPEC.md) — product behavior, data boundaries, safety,
  failure handling, acceptance criteria, and delivery plan.
- [`TECHSTACK.md`](TECHSTACK.md) — architecture, technology choices, service
  ownership, and the authoritative target repository structure.
- [`DATASETS.md`](DATASETS.md) — data currently available, its provenance, and
  identified gaps.
- [`docs/design/visual-system.md`](docs/design/visual-system.md) — the GridOS
  golden visual and motion system and the immersive-web case studies behind it.

Supporting domain reasoning and source investigations live under [`docs/`](docs/).
Large historical downloads remain in the ignored `data/` research cache; only
small reviewed samples belong in `testdata/fixtures/`.

## Repository rule

Keep the repository lean: no empty architecture directories, speculative
services, generated research artifacts, or large raw-data downloads in version
control. Unit tests live with their owning code; only cross-service tests and
small deterministic fixtures receive shared top-level locations.

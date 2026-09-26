# GridOS

GridOS coordinates a distributed fleet of residential batteries as dependable
grid capacity while protecting each household's backup reserve. It forecasts
conditions, builds and validates safe dispatch plans, executes them through a
failure-tolerant workflow, and verifies the power actually delivered.
Members can choose a resilience plan or schedule Travel Flex; additional
capacity is used only when safety protections hold and conservative incremental
margin remains positive.

## Current status

The repository is in the specification and data-research stage. No application
has been implemented yet, and planned directories are created only when real
code needs them.

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

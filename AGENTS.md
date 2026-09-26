# AGENTS.md

**Solve every task with the least code and the simplest, highest-quality architecture. Build the most optimal solution. Ensure maximal correctness.**

Binding for every change of every size. No exemptions for urgency, prototypes, or temporary code.

When these conflict, resolve only in this order:

1. Maximal correctness. It must actually work, including edges and failures. Verify with a re-runnable check before claiming done.
2. Simplest highest-quality architecture. Fewest concepts, shortest correct path, idiomatic, no extra layers.
3. Least code. Remove the need for code, never safeguards or clarity.
4. Optimal over the life of the code. Easiest to understand, change, and delete. Speed last and only with measurement.

## Before adding code

1. Add a concept, layer, parameter, dependency, or helper only for a concrete need in the current task.
2. Use existing code, the standard library, or the framework's intended path when sufficient.
3. Introduce a reusable abstraction only for three real, divergent uses or a true external boundary. Implement one-off behavior directly.
4. Prefer changes that are easy to reverse. Reject complexity justified only by hypothetical future needs.

## Shape

- Flat over nested, single path over branching. Guard clauses and linear pipelines. Branch only for required behavior. Domain values instead of boolean mode parameters.
- Parse at the edge, trust the interior. Validate untrusted input at network, disk, and user boundaries with a real schema. Never mask an invariant violation with a silent fallback.
- Make illegal states unrepresentable. Mutually exclusive states are closed unions, one-ofs, or state machines. Keep independent facts independent.
- Deep modules over shallow wrappers. Collocate models, lifecycle, and flow at the site of use. No util, helper, misc, or shared folders.
- Keep every feature deletable without unrelated changes or residual glue.
- No `any` in TypeScript, no `Any` in Python, no `interface{}` as an escape hatch in Go.

## What simplicity never cuts

Authorization at every boundary. Idempotency keys on every command. Timeouts and bounded retries. Audit rows for every state transition. Validation at the edge. Migrations with a rollback path. A household reserve is never relaxed to satisfy a target.

## Comments: zero

Write zero comments or docstrings. If code needs explaining, rename or restructure it until it does not. This includes inline and block comments, doc blocks, JSX comments, commented-out code, and divider banners. Machine directives (`//go:`, `#!`, `// Code generated`) are the only exception. The pre-commit hook rejects any other added comment line.

## Gates are one-way

The hooks in `tools/development/hooks` are the reviewer. Red means not done: report the failing output. Never loosen a ceiling, add a suppression (`nolint`, `noqa`, `eslint-disable`, `ts-ignore`, `type: ignore`), weaken a hook, or pass `--no-verify`. Over-ceiling code means extract along a real seam.

Ceilings, enforced by the hook on staged files: cyclomatic complexity 18, nesting depth 4, 500 lines per file, 150 lines per function.

## The gate is fast, and only fast

The pre-commit hook runs format, lint, and the tests that belong to the files you staged. Nothing else. It does not run the whole suite, does not start containers, and finishes in seconds. The full suite runs once per wave, by the director, at the wave gate. Do not run the full suite on your own; do not re-run a passing check.

Install once: `git config core.hooksPath tools/development/hooks`

## Tests

- Engines, validators, math, state machines, storage transitions, generators: write the failing test first, confirm it fails for the named reason, then the minimum code to pass. RED and GREEN are separate commits. A RED commit stages only test files and the hook skips test execution for it; a commit that stages implementation must be green.
- A negative assertion needs a positive control proving the path is live. A test that fails by typo proves nothing.
- Unit tests live beside their code. Only cross-service behavior goes under `tests/`.
- State the exact command and its last line with every result. A fast-gate result is never reported as the full gate.

## Commits

- Many small commits. One per RED, one per GREEN, one per refactor. Do not batch a day of work.
- Subject: imperative, capitalized, under 72 characters, no prefix, no trailing period. Body says why, not what. The commit-msg hook enforces the subject.
- Shared tree, direct to `main`. Commit exact paths: `git commit -- <paths>`. Never `git add -A`, never reset, stash, or checkout in the shared tree, never delete `.git/index.lock`. An untracked file may belong to another live agent.
- Edit only inside the paths your lane owns in `claude docs/BUILD_ORDER.md`. Request anything else through the director.

## Code is the source of truth

Do not create documentation, planning notes, decision records, summaries, or hand-off files unless a plan item names one. Report progress in the mailbox line format from the build order.

## Stack

Go 1.23+ for `services/control` and `services/gateway-simulator`. Python 3.12 with `uv` for `services/decision` and `tools/`. TypeScript with `pnpm` for `apps/console`. Protobuf managed by `buf` in `contracts/`. PostgreSQL 16 and Temporal via `infrastructure/local/compose.yaml`. Product behavior is `FULL_SPEC.md`; architecture and layout are `TECHSTACK.md`; data provenance is `DATASETS.md`. Commands live in the root `Makefile` once it exists.

Installed toolchain: Go 1.27.1, buf 1.73.0, Bazelisk 1.29.0 with Bazel 9.2.0, Temporal CLI 1.9.1, uv 0.11.7 with CPython 3.12.13, sqlc 1.31.1.

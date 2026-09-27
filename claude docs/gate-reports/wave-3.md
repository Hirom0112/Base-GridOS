# Gate 3 report

**Run:** 2026-09-27 by the director on `main`.
**Result:** backend GREEN; the UI leg is open on the console track (same
condition as Gate 2, steps owed by the console).
**Verified items:** 32 of 34 in Wave 3. Open: 3F.3 (fixture recording,
underway on the ready demo) and 3F.5 (the UI leg). Lanes 3A, 3B, 3C, 3D,
3E complete; 3F has 3F.1, 3F.2, 3F.4 verified.

## Canonical scenario and budgets

```text
$ uv run --project services/decision pytest services/decision -k perf_5000 --durations=1
1 passed (1.45s call)                                   plan under 10 s
$ go test ./services/control/internal/safety/ -run '^$' -bench Validate5000x288 -benchtime 3x
BenchmarkValidate5000x288-14   3   281913889 ns/op       validation under 2 s
$ go test ./tests/integration/ -run 'HeatEventCanonical|InfeasibleTargetShortfall|OptimizerTimeoutFallback'
--- PASS: TestHeatEventCanonical (110.72s)
--- PASS: TestInfeasibleTargetShortfall (110.04s)
--- PASS: TestOptimizerTimeoutFallback (101.64s)
```

The timeout scenario starts the decision server with a tiny solver budget so
HiGHS times out deterministically and the validated fallback is served with
PLAN_FALLBACK_SELECTED; the infeasible scenario shows a per-interval
shortfall with every reserve intact.

## Differential, replay, isolation

```text
$ go test ./services/control/internal/safety/ -run Golden -count=1
ok  	github.com/Hirom0112/Base-GridOS/services/control/internal/safety	0.299s   (36 subtests, each fixture with reserve and power rejection controls)
$ go run ./services/control/cmd/replay --event event_dir_1790468949            (standing demo event, manifest with commit hash)
IDENTICAL
$ go test ./services/control/internal/analytics/ -run Isolation -count=1
ok  	github.com/Hirom0112/Base-GridOS/services/control/internal/analytics	0.605s
```

## Decision service and contracts

```text
services/decision: 71 passed; mypy: Success: no issues found in 23 source files
buf lint + buf breaking against main: contracts ok
```

Forecasting (3A) ships a similar-day site baseline, regional load and price
persistence, county-hour outage risk marked modeled_estimate, device
availability with SOC intervals, and an evaluation harness with a
deterministic baseline. The optimizer (3B) is a discharge-only HiGHS LP over
the event window with cohorts, disaggregation, explanations, a bounded
runtime, hypothesis invariants, and solver-first serving with fallback. The
control plane (3D) freezes the forecast inside the input snapshot, binds
approval to a digest of frozen inputs, exposes plan explanation and unsafe
alternative validation, and replaces dropped devices through the safety
gate with a new plan version.

## UI leg

`playwright test demo-path` against the standing demo: steps 2, 3, 9, 13,
16 pass; the rest fail on missing console elements. Gate 3 asks for every
step except 1 and 7; that condition stays open on the console track and
closes when those screens land.

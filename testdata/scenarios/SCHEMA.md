# Scenario schema

Scenario files are YAML documents validated by `tools.generation.scenario.model.Scenario`.

| Field | Type | Meaning |
| --- | --- | --- |
| `name` | nonempty string | Stable scenario name |
| `provenance` | `SIMULATED` | Fixture provenance |
| `clock.seed` | integer | Replay seed |
| `clock.start_at` | timezone-aware timestamp | Initial simulated time |
| `clock.interval_seconds` | positive integer | Clock step |
| `fleet.path` | path | JSONL fleet reference |
| `fleet.size` | positive integer | Expected fleet records |
| `event.region` | nonempty string | Addressable event region |
| `event.start_at`, `event.end_at` | timezone-aware timestamps | Forward event window |
| `event.target_mw` | positive number | Requested service |
| `event.boundary` | boundary enum | Measurement boundary used by planning and verification |
| `injections[].at` | timezone-aware timestamp | Seeded injection time |
| `injections[].kind` | injection enum | Failure to inject |
| `expected.final_event_state` | `VERIFIED`, `RECONCILED`, or `REPORTED` | Required terminal progress |
| `expected.reserve_violations` | nonnegative integer | Allowed reserve violations |
| `expected.allows_shortfall` | boolean | Whether safe shortfall is acceptable |
| `expected.required_recovery_actions` | recovery-action list | Actions the run must demonstrate |

Measurement boundaries are `BATTERY_TERMINAL`, `METER_NET_EXPORT`, and `IMPORT_REDUCTION_VS_BASELINE`.

Injection kinds are `OFFLINE_DEVICES`, `DELAYED_GATEWAY`, `DELAYED_TELEMETRY`, `DROPPED_MESSAGES`, `DUPLICATED_MESSAGES`, `GATEWAY_RESTART`, `WORKER_RESTART`, `PARTIAL_REGION_OUTAGE`, `BAD_FORECASTS`, `HOT_BATTERIES`, `STALE_STATE`, and `OPTIMIZER_TIMEOUT`.

Recovery actions are `RETRY`, `REMOVE_STALE_CAPACITY`, and `REBALANCE`. The clock must start no later than the event. Every injection must fall between clock start and event end.

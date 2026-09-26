# GridOS truth model

## Event measurement

Every event declares one measurement boundary. The canonical event uses `METER_NET_EXPORT`; `BATTERY_TERMINAL` is selectable. Dispatch planning and delivery verification use the same boundary for the entire event. Grid service is positive: battery discharge, meter export, and import reduction are positive at their respective boundaries. Charge, meter import, and increased import are negative.

The planning and reporting interval is five minutes, so `dt = 1/12` hour. Delivery is integrated over event time from observations at the declared boundary. Missing or stale observations are unknown, never zero. Reports retain signed error, absolute error, completeness, and uncertainty instead of turning a numeric tolerance into a binary claim.

## Physical state

Charge power `p_ch` and discharge power `p_dis` are nonnegative AC-side values. They cannot both be positive in one interval.

```text
energy_next_kWh = energy_kWh
                  + charge_efficiency * p_ch_kW * dt
                  - p_dis_kW * dt / discharge_efficiency

effective_reserve_kWh <= energy_kWh <= usable_capacity_kWh
0 <= p_ch_kW <= permitted_charge_kW
0 <= p_dis_kW <= permitted_discharge_kW
```

The effective reserve is `max(hardware_floor, plan_floor, dynamic_override)`. A zero-percent plan floor removes only the member-selected buffer. It never removes the hardware floor or an active safety override. A grid-connected dispatch must preserve the effective reserve. During an outage, backup service may consume energy below the plan floor while continuing to respect the hardware floor.

Backup duration is always reported both at current usage and at a 750 W reference critical load. Each value is AC energy available above the applicable hardware floor multiplied by discharge efficiency, divided by the respective positive load. A zero or unknown current load produces an unknown duration, not infinity.

## Site operating states

| Current state | Input | Next state | Grid-service capacity |
| --- | --- | --- | --- |
| `ON_GRID` | Grid outage detected | `OFF_GRID_OUTAGE` | Zero |
| `OFF_GRID_OUTAGE` | Home power cannot be served | `OFF_GRID_NO_HOME_POWER` | Zero |
| `OFF_GRID_OUTAGE` | Load exceeds overcurrent limit | `OFF_GRID_OVERCURRENT` | Zero |
| `OFF_GRID_OVERCURRENT` | Protection enters standby | `OFF_GRID_OVERCURRENT_STANDBY` | Zero |
| Any available state | Telemetry becomes unavailable | `TELEMETRY_UNAVAILABLE` | Zero |
| Any off-grid state | Grid returns and connection is verified | `ON_GRID` | Recomputed |
| `TELEMETRY_UNAVAILABLE` | Fresh telemetry identifies an off-grid state | Identified off-grid state | Zero |
| `TELEMETRY_UNAVAILABLE` | Fresh telemetry verifies grid connection | `ON_GRID` | Recomputed |

Every islanded state has zero grid-service capacity regardless of stored energy. `TELEMETRY_UNAVAILABLE` fails closed until fresh telemetry identifies the physical state.

## Command semantics

A command has an immutable command ID and idempotency key, event and device IDs, plan version, monotonic generation, absolute setpoint, effective time, expiry, and policy version. Retrying repeats the same ID and byte-equivalent payload. A changed schedule or cancellation uses a new ID and a higher generation. Receivers durably deduplicate IDs, reject conflicting reuse, reject obsolete generations, and reject commands received after expiry. Expiry enforces a safe zero grid-service setpoint. Reconnection never revives an expired command.

| Current state | Input | Next state |
| --- | --- | --- |
| `PERSISTED` | Send begins before expiry | `SENT` |
| `PERSISTED` | Expiry | `EXPIRED` |
| `SENT` | Receipt accepted | `ACKNOWLEDGED` |
| `SENT` | Acknowledgement deadline passes | `UNCERTAIN` |
| `SENT` | Expiry | `EXPIRED` |
| `ACKNOWLEDGED` | Effective time reached | `EXECUTING` |
| `UNCERTAIN` | Fresh telemetry proves execution | `EXECUTING` |
| `UNCERTAIN` | Durable receiver state proves rejection | `REJECTED` |
| `ACKNOWLEDGED`, `UNCERTAIN`, or `EXECUTING` | Higher generation cancellation accepted | `CANCELLED` |
| `ACKNOWLEDGED`, `UNCERTAIN`, or `EXECUTING` | Expiry | `EXPIRED` |
| `EXECUTING` | Measurement window closes | `COMPLETED` |
| Any nonterminal state | Receiver rejects the command | `REJECTED` |

Acknowledgement proves receipt, not execution or delivery. `UNCERTAIN` capacity remains bounded as possibly active until telemetry, durable receiver state, cancellation, or expiry resolves it.

## Event lifecycle

| Current state | Completed transition | Next state |
| --- | --- | --- |
| — | Request accepted | `REQUESTED` |
| `REQUESTED` | Versioned plan created | `PLANNED` |
| `PLANNED` | Independent safety validation passes | `VALIDATED` |
| `VALIDATED` | Required operator approval recorded | `APPROVED` |
| `APPROVED` | Command intents and outbox entries commit | `COMMANDS_PERSISTED` |
| `COMMANDS_PERSISTED` | Send attempts begin | `SENT` |
| `SENT` | Every command is acknowledged or explicitly uncertain | `ACKNOWLEDGED_OR_UNCERTAIN` |
| `ACKNOWLEDGED_OR_UNCERTAIN` | Event window begins | `EXECUTING` |
| `EXECUTING` | Boundary measurements are evaluated | `VERIFIED` |
| `VERIFIED` | Commands, late data, and uncertainty are resolved | `RECONCILED` |
| `RECONCILED` | Versioned event report is published | `REPORTED` |

Replanning creates a new plan version but does not move an event backward. Unsafe or infeasible work ends in a quantified shortfall and audit record rather than skipping a lifecycle state or relaxing reserve.

# GridOS threat model

## Scope and trust boundaries

The local demo uses generated devices, simulated households, public fixtures, a shared gateway token, and a local step-up signing key. Its role and member headers are caller supplied. Operational household data and live commands remain outside this trust model until deployment identity, transport encryption, secret storage, retention, and access review are approved.

| Boundary | Main failure | Required control |
| --- | --- | --- |
| Browser to API | A user forges a role, member ID, or exact-location permission | Authenticate the person at the API, bind role, member, and capability claims to that identity, then authorize every RPC. The local headers are demo stubs. |
| API to workflow | A replay or stale approval changes an event | Bind the signed step-up assertion to action, event, plan version, and a five-minute expiry; reject reused nonces; use command idempotency keys and durable event transitions. |
| Workflow to outbox | A crash sends a command before persistence or sends it twice | Persist intent before delivery, use the same command ID on retry, and audit each transition. |
| Outbox to gateway | A forged sender or receipt drives a device | Authenticate each gateway and control connection, rotate per-gateway credentials, verify receipts, and keep the hardware reserve independent of a member preference. The shared demo token is not an operational credential. |
| Control to analytics | Household identifiers or command credentials leak through telemetry, logs, traces, or reports | Scrub exported fields, aggregate partner and public output, restrict raw access, and test exporter output. |
| Data source to control | A simulated or restricted datum is treated as verified public evidence | Preserve provenance and dataset license terms; reject unsupported relabeling and redistribution. |

## Control-plane authorization

Every control RPC has a server-side role check or, for gateway telemetry, a gateway credential check. Operator, approver, analyst, partner, service, and member rights are separate. Member access also checks the bound member and site in storage. The role matrix test covers each handler and missing or unknown roles. The local `X-GridOS-Role`, member ID, and permissions headers can be forged; production authorization requires verified identity claims at the API boundary.

## Dispatch step-up and audit

Approval and emergency stop require an HMAC-signed, action-bound assertion when `GRIDOS_STEP_UP_KEY` is configured. The server takes the actor from the assertion, rejects expiry and replay, and writes an immutable acceptance audit row. The nonce lives in the assertion record while event audit rows carry the event correlation ID. The local key and the unset-key warning are marked `STUBBED`; production must use identity-provider assertions and fail closed when they are unavailable.

## Transport and stored secrets

The local HTTP and shared gateway token are for generated data only. Production must terminate authenticated TLS on browser, service, and gateway paths, use managed workload identity or short-lived credentials, encrypt stored secrets, and rotate credentials. The pre-commit secrets scan rejects staged credentials; deployment must keep real values out of the repository and logs.

## Household minimization and partner views

Collect only fields needed for dispatch, verification, consent, and member features. Keep device and household identifiers inside authorized service paths. Fleet and geographic partner views aggregate locations, and partner reports remove site identifiers. Do not export raw household telemetry to partner or public views.

## Exact site location

Exact H3 cells and site drill-down require the separate `site_location` capability. Production identity must bind that capability to an approved principal and record access; a local permissions header alone is insufficient.

## Retention, deletion, access review, and incidents

Apply [retention.md](retention.md) before accepting operational household data. Restrict database and backup access to named duties, review grants on a defined schedule, revoke access when duties change, and preserve audit evidence during an incident. An incident response owner must isolate credentials, stop command delivery if needed, assess affected data, restore from a verified state, and record the timeline and notifications.

## Consent and policy versions

Freeze the selected plan, consent version, catalog terms, risk policy, and reserve floor used for each decision. An expired or unconsented preference cannot lower a household reserve. Keep the recorded version with the event report and audit so replay does not silently apply a later policy.

## Travel and away state

Travel windows, away periods, energy baselines, and anomaly preferences reveal household behavior. Only the bound member and the services performing the consented action may read them. Exclude them from partner views, traces, and analytics; delete or aggregate them under the household schedule in [retention.md](retention.md). Describe away alerts as energy anomalies, never verified intrusions.

## Logs, traces, analytics, and errors

Scrub private site IDs, travel windows, away state, command credentials, and member identifiers before exporting logs, traces, analytics, or error reports. Use correlation and workflow IDs only when their mapping stays in the restricted control database. Verify each configured exporter, not only the scrubber function.

## Source licenses and redistribution

Use [DATASETS.md](../../../DATASETS.md) as the source catalog. Check each license and redistribution condition before publishing a fixture, partner report, or derived feed. Preserve source name, timestamp, and provenance through forecasting, margin calculation, and reports.

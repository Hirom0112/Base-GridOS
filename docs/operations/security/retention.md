# Household data retention and deletion

## Admission rule

GridOS may use generated households and licensed public fixtures in the local demo. Do not connect operational household, gateway, or member data until the data owner approves a field-level retention schedule, lawful purpose, access owner, backup expiry, and deletion procedure, and the deployment enforces them. A missing decision is a deployment block, not permission for indefinite retention.

## Current retention behavior and open decisions

| Data | Current behavior | Decision or enforcement required before operational use |
| --- | --- | --- |
| Raw telemetry observations | Stored by UTC day. Ingest rejects observations older than the current day plus six prior UTC dates. `TelemetryStore.Prune` drops old partitions and batches deletion from the default partition when invoked. | Name the prune scheduler and alert on missed runs. Approve whether the seven-UTC-date window meets the operational purpose and set a backup expiry. |
| Event reports, reward ledger, audit journal, and accepted step-up assertions | Rows are append-only. Reports and audit remain for replay; no expiry or member deletion exists. | Data owner and legal owner approve a retention period, minimal identifiers, legal hold rule, and a way to satisfy deletion requests without mutating the audit chain. |
| Member and site bindings, preferences, consent, offers, and selected plans | Effective dates and consent versions control use, but stored rows have no deletion schedule. | Approve a purpose-specific period after a plan ends or membership ends, plus the deletion and backup process. |
| Travel Flex windows and away periods | Consent and effective dates control active use. End commands set cancellation or end timestamps. The full rows remain in storage after the active window ends. | Approve a short post-end period; implement deletion or minimization so only required audit evidence survives. No partner export of schedules or away state. |
| Away baselines, anomaly preferences, and home activity alerts | Bound to the member and used for consented anomaly decisions; no approved deletion schedule exists. | Approve periods after opt-out or end of away state, remove raw baselines and alerts on schedule, and limit audit evidence to consent/version and decision metadata. |
| Backups, local fixtures, logs, traces, and analytics exports | Generated fixtures are checked in; operational backup and export expiry is not defined. Scrubbers exist for exported sensitive fields. | Approve backup lifetime and restore isolation, inventory every export, and verify scrubbers are wired before operational traffic. |

The seven-date raw-telemetry window is the only automatic source-data retention bound above. Pruning runs on invocation and may lag if the job stops. Effective dates stop use of Travel Flex and away periods; they do not delete the stored row.

## Member deletion flow

1. Authenticate the requester, verify membership and site binding, record the request and any legal hold, and block new member-specific processing while the request is resolved.
2. Inventory member bindings, consent, preferences, offers, selected plans, Travel Flex, away periods, baselines, alerts, telemetry device mappings, reward entries, reports, and exports. Identify shared event records separately from member-only records.
3. End active Travel Flex and away state and revoke future consented processing. Apply the approved schedule to delete or minimize member-only rows and exports. Keep only the minimum immutable audit evidence required by the approved legal hold and retention decision.
4. Expire backups and caches under the approved backup window, prevent a restore from recreating deleted active records, and verify the member's data is absent from active queries and partner views.
5. Record the deletion decision, affected stores, verifier, timestamp, and any retained legal-hold material without copying travel times, away signals, or command credentials into the deletion log.

This flow is a production requirement. The current build does not implement complete household deletion or backup expiry, so no operational household data may be admitted yet.

## Access and incident review

The data owner must assign an access reviewer and review interval before rollout. Review member-data readers, exact-location capability, database administrators, analytics exporters, and backup restores; revoke unused grants. On a suspected leak, stop affected exports and command delivery as needed, rotate the relevant credentials, preserve minimal evidence, identify affected subjects and datasets, and follow the approved notification procedure. The incident owner and notification deadlines remain deployment decisions.

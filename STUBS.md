# Stubs

Every `PENDING-LIVE` marker in the tree, kept equal to the tree by the
director. Each marks a local substitute awaiting a named live system.

| Marker | Where | What it stands in for | Retired by |
| --- | --- | --- | --- |
| `PENDING-LIVE` | `tools/development/mockapi/main.go` (`localAuthStatus`), shown by the console badge in `apps/console/src/api/auth.tsx` | Local dev identities and roles for the mock API in place of Clerk sessions | Clerk sessions or the mock's retirement |
| `PENDING-LIVE` | `tools/development/mockapi/main.go` (`localStepUpStatus`) | Server-signed local approval and emergency-stop assertions in place of identity-provider step-up | Clerk action-bound step-up assertions |
| `PENDING-LIVE` | `services/control/cmd/control/main.go` (`LOCAL_GATEWAY_CREDENTIAL_STATUS`) | Default local bearer token for gateway telemetry in place of issued gateway credentials | An approved gateway identity provider and secret store |
| `PENDING-LIVE` | `services/control/cmd/control/main.go` (`LOCAL_STEP_UP_STATUS`) | Local HMAC step-up key, or an explicit startup warning while the key is unset, in place of identity-provider assertions | Clerk step-up assertions and an approved secret store |
| `PENDING-LIVE` | `testdata/fixtures/api/ReportService/GetEventReport.json`, `GetEventReport.comparison_peer.json`, `GetEventReport.partner.json`, `CompareEventReports.json` (`event-report-a`, `event-report-b`) | Hand-built reports carrying a sourced reward and a modeled margin; the recorded live report is `GetEventReport.live.json` | A live event with a negative LZ_AEN price and an event-scoped reward offer |

Simulated fleet and public context carry no marker: they enter through
`GRIDOS_FLEET` (`services/control/cmd/control/main.go`,
`services/control/cmd/worker/main.go`) and `GRIDOS_PUBLIC_CONTEXT_DIR`
(`services/control/cmd/control/main.go`), and a live feed replaces the file
those variables name.

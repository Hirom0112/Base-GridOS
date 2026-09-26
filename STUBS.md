# Stubs

Every `STUBBED` or `PENDING-LIVE` marker in the tree, kept equal to the tree
by the director. A marked stub is honest; a silent one is not.

| Marker | Where | What it stands in for | Retired by |
| --- | --- | --- | --- |
| `STUBBED` | `tools/development/mockapi/main.go` (`localAuthStatus`) | Local dev identities and roles for the mock API in place of Clerk sessions | An authorized identity provider on the mock, or the mock's retirement |
| `STUBBED` | `services/control/cmd/control/main.go` (`LOCAL_GATEWAY_CREDENTIAL_STATUS`) | Default local bearer token for gateway telemetry in place of issued gateway credentials | Wave 5 connector work (5A.1) or an authorized gateway identity |


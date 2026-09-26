# Stubs

Every `STUBBED` or `PENDING-LIVE` marker in the tree, kept equal to the tree
by the director. A marked stub is honest; a silent one is not.

| Marker | Where | What it stands in for | Retired by |
| --- | --- | --- | --- |
| `STUBBED` | `tools/development/mockapi/main.go` (`localAuthStatus`) | Local dev identities and roles for the mock API in place of Clerk sessions | An authorized identity provider on the mock, or the mock's retirement |
| `REPLACED-IN-WAVE-2` | `services/control/internal/api/dispatcher.go` (`DispatcherLifecycle`) | Straight-line Phase 1 dispatcher in place of the Temporal workflow | Item 2B.7 |


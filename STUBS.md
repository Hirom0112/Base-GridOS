# Stubs

Every `STUBBED` or `PENDING-LIVE` marker in the tree, kept equal to the tree
by the director. A marked stub is honest; a silent one is not.

| Marker | Where | What it stands in for | Retired by |
| --- | --- | --- | --- |
| `STUBBED` | `tools/development/mockapi/main.go` (`localAuthStatus`) | Local dev identities and roles for the mock API in place of Clerk sessions | An authorized identity provider on the mock, or the mock's retirement |
| `STUBBED` | `services/control/cmd/control/main.go` (`LOCAL_GATEWAY_CREDENTIAL_STATUS`) | Default local bearer token for gateway telemetry in place of issued gateway credentials | Wave 5 connector work (5A.1) or an authorized gateway identity |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`MarketLiveSlot`) | Simulated market prices and regional load | A validated live market feed |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`WeatherLiveSlot`) | Simulated weather and alerts | A refreshed NWS feed |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`OutageLiveSlot`) | Simulated outage risk | An approved current outage feed |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`HouseholdLoadLiveSlot`) | Simulated household load | An authorized interval meter or gateway feed |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`BatteryLiveSlot`) | Simulated battery telemetry | An authorized device stream |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`TopologyLiveSlot`) | Simulated grid topology | An approved operational network mapping |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`MemberPolicyLiveSlot`) | Simulated member reserve and consent | A consented member configuration feed |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`BehaviorLiveSlot`) | Simulated resilience and Travel Flex behavior | Consented settings and observed outcomes |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`CommandsLiveSlot`) | Simulated command acceptance | An authenticated device command API |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`SettlementLiveSlot`) | Simulated settlement economics | Contract rules and actual statements |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`PricingLiveSlot`) | Simulated pricing and rewards | Authorized catalog, agreements, and reward ledger |

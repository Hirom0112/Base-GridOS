# Stubs

Every `STUBBED` or `PENDING-LIVE` marker in the tree, kept equal to the tree
by the director. A marked stub is honest; a silent one is not.

| Marker | Where | What it stands in for | Retired by |
| --- | --- | --- | --- |
| `STUBBED` | `tools/development/mockapi/main.go` (`localAuthStatus`), shown by the console badge in `apps/console/src/api/auth.tsx` | Local dev identities and roles for the mock API in place of Clerk sessions | An authorized identity provider on the mock, or the mock's retirement |
| `STUBBED` | `tools/development/mockapi/main.go` (`localStepUpStatus`) | Server-signed local approval and emergency-stop assertions in place of identity-provider step-up | An authorized identity provider with action-bound assertions |
| `STUBBED` | `services/control/cmd/control/main.go` (`LOCAL_GATEWAY_CREDENTIAL_STATUS`) | Default local bearer token for gateway telemetry in place of issued gateway credentials | Wave 5 connector work (5A.1) or an authorized gateway identity |
| `STUBBED` | `services/control/cmd/control/main.go` (`LOCAL_STEP_UP_STATUS`) | Local HMAC step-up key, or an explicit startup warning while the key is unset, in place of identity-provider assertions | An authorized identity provider and operator approval policy |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`MarketLiveSlot`) | Simulated market prices and regional load | A validated live market feed |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`WeatherLiveSlot`) | Simulated weather and alerts | A refreshed NWS feed |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`OutageLiveSlot`) | Simulated outage risk | An approved current outage feed |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`HouseholdLoadLiveSlot`) | Simulated household load | An authorized interval meter or gateway feed |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`BatteryLiveSlot`) | Simulated battery telemetry | An authorized device stream |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`TopologyLiveSlot`) | Simulated grid topology | An approved operational network mapping |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`MemberPolicyLiveSlot`) | Simulated member reserve and consent; MEMBER_AUTHORIZED site bindings wait for a live identity entitlement source, while only SIMULATED bindings exist today | A consented member configuration and identity entitlement feed |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`BehaviorLiveSlot`) | Simulated resilience and Travel Flex behavior | Consented settings and observed outcomes |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`CommandsLiveSlot`) | Simulated command acceptance | An authenticated device command API |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`SettlementLiveSlot`) | Simulated settlement economics | Contract rules and actual statements |
| `PENDING-LIVE` | `services/control/internal/connectors/connectors.go` (`PricingLiveSlot`) | Simulated pricing and rewards | Authorized catalog, agreements, and reward ledger |
| `PENDING-LIVE` | `services/control/internal/connectors/notification.go` (`NotificationLiveSlot`) | Simulated member alert delivery rows; no live channel is authorized | A consented and authorized notification channel with delivery evidence |

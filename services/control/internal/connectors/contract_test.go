package connectors

import (
	"context"
	"testing"
	"time"
)

func TestConnectorSchemaContract(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	sim := NewSimulated(Data{
		Markets:     map[string]MarketSnapshot{"houston": {Region: "houston", PriceUSDPerMWh: 80, LoadMW: 1000, At: at}},
		Weather:     map[string]WeatherSnapshot{"houston": {Region: "houston", TemperatureC: 34, Alert: "heat", At: at}},
		Outages:     map[string]OutageSnapshot{"houston": {Region: "houston", Probability: 0.2, At: at}},
		Loads:       map[string]LoadSnapshot{"site-1": {SiteID: "site-1", HomeKW: 2, SolarKW: 1, At: at}},
		Batteries:   map[string]BatterySnapshot{"device-1": {DeviceID: "device-1", SOC: 0.7, PowerKW: 1, At: at}},
		Topology:    map[string]TopologySnapshot{"site-1": {SiteID: "site-1", FeederID: "feeder-1"}},
		Policies:    map[string]PolicySnapshot{"member-1": {MemberID: "member-1", Reserve: 0.4, Consented: true}},
		Behavior:    map[string]BehaviorSnapshot{"member-1": {MemberID: "member-1", ResilienceTier: "BALANCED", TravelFlexActive: true}},
		Settlements: map[string]SettlementSnapshot{"event-1": {EventID: "event-1", AmountCents: 1200}},
		Pricing:     map[string]PricingSnapshot{"member-1": {MemberID: "member-1", CatalogVersion: "v1", RewardCents: 500}},
	})
	live := newFutureLiveFixture()
	ctx := context.Background()
	assertContract(t, "market", sim.Market, live.Market, ctx, "houston")
	assertContract(t, "weather", sim.WeatherAndAlerts, live.WeatherAndAlerts, ctx, "houston")
	assertContract(t, "outage", sim.OutageRisk, live.OutageRisk, ctx, "houston")
	assertContract(t, "household load", sim.HouseholdLoad, live.HouseholdLoad, ctx, "site-1")
	assertContract(t, "battery", sim.BatteryTelemetry, live.BatteryTelemetry, ctx, "device-1")
	assertContract(t, "topology", sim.GridTopology, live.GridTopology, ctx, "site-1")
	assertContract(t, "policy", sim.MemberPolicy, live.MemberPolicy, ctx, "member-1")
	assertContract(t, "behavior", sim.ResilienceAndTravelFlex, live.ResilienceAndTravelFlex, ctx, "member-1")
	assertContract(t, "settlement", sim.Settlement, live.Settlement, ctx, "event-1")
	assertContract(t, "pricing", sim.PricingAndRewards, live.PricingAndRewards, ctx, "member-1")
	command := CommandRequest{ID: "command-1", DeviceID: "device-1", PowerKW: 1, ExpiresAt: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)}
	simReceipt, simErr := sim.SendCommand(ctx, command)
	liveReceipt, liveErr := live.SendCommand(ctx, command)
	if simErr != nil || liveErr != nil || simReceipt != liveReceipt {
		t.Fatalf("command schema: simulated=%+v %v live=%+v %v", simReceipt, simErr, liveReceipt, liveErr)
	}
}

func assertContract[T comparable](t *testing.T, name string, simulated, live func(context.Context, string) (T, error), ctx context.Context, key string) {
	t.Helper()
	simulatedValue, simulatedErr := simulated(ctx, key)
	liveValue, liveErr := live(ctx, key)
	if simulatedErr != nil || liveErr != nil || simulatedValue != liveValue {
		t.Fatalf("%s schema: simulated=%+v %v live=%+v %v", name, simulatedValue, simulatedErr, liveValue, liveErr)
	}
}

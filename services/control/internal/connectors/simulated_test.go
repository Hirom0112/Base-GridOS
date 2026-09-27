package connectors

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSimulatedConnectorRows(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	sim := NewSimulated(Data{
		Markets:     map[string]MarketSnapshot{"houston": {Region: "houston", PriceUSDPerMWh: 80, LoadMW: 1000, At: now}},
		Weather:     map[string]WeatherSnapshot{"houston": {Region: "houston", TemperatureC: 34, Alert: "heat", At: now}},
		Outages:     map[string]OutageSnapshot{"houston": {Region: "houston", Probability: 0.2, At: now}},
		Loads:       map[string]LoadSnapshot{"site-1": {SiteID: "site-1", HomeKW: 2, SolarKW: 1, At: now}},
		Batteries:   map[string]BatterySnapshot{"device-1": {DeviceID: "device-1", SOC: 0.7, PowerKW: 1, At: now}},
		Topology:    map[string]TopologySnapshot{"site-1": {SiteID: "site-1", FeederID: "feeder-1"}},
		Policies:    map[string]PolicySnapshot{"member-1": {MemberID: "member-1", Reserve: 0.4, Consented: true}},
		Behavior:    map[string]BehaviorSnapshot{"member-1": {MemberID: "member-1", ResilienceTier: "BALANCED", TravelFlexActive: true}},
		Settlements: map[string]SettlementSnapshot{"event-1": {EventID: "event-1", AmountCents: 1200}},
		Pricing:     map[string]PricingSnapshot{"member-1": {MemberID: "member-1", CatalogVersion: "v1", RewardCents: 500}},
	})
	assertPublicSnapshots(t, sim)
	assertSiteSnapshots(t, sim)
	assertCommandsAndFailures(t, sim, now)
}

func assertPublicSnapshots(t *testing.T, sim *Simulated) {
	t.Helper()
	ctx := context.Background()
	if got, err := sim.Market(ctx, "houston"); err != nil || got.LoadMW != 1000 {
		t.Fatalf("market: %+v, %v", got, err)
	}
	if got, err := sim.WeatherAndAlerts(ctx, "houston"); err != nil || got.Alert != "heat" {
		t.Fatalf("weather: %+v, %v", got, err)
	}
	if got, err := sim.OutageRisk(ctx, "houston"); err != nil || got.Probability != 0.2 {
		t.Fatalf("outage: %+v, %v", got, err)
	}
}

func assertSiteSnapshots(t *testing.T, sim *Simulated) {
	t.Helper()
	ctx := context.Background()
	if got, err := sim.HouseholdLoad(ctx, "site-1"); err != nil || got.HomeKW != 2 {
		t.Fatalf("load: %+v, %v", got, err)
	}
	if got, err := sim.BatteryTelemetry(ctx, "device-1"); err != nil || got.SOC != 0.7 {
		t.Fatalf("battery: %+v, %v", got, err)
	}
	if got, err := sim.GridTopology(ctx, "site-1"); err != nil || got.FeederID != "feeder-1" {
		t.Fatalf("topology: %+v, %v", got, err)
	}
	if got, err := sim.MemberPolicy(ctx, "member-1"); err != nil || got.Reserve != 0.4 {
		t.Fatalf("policy: %+v, %v", got, err)
	}
	if got, err := sim.ResilienceAndTravelFlex(ctx, "member-1"); err != nil || !got.TravelFlexActive {
		t.Fatalf("behavior: %+v, %v", got, err)
	}
	if got, err := sim.Settlement(ctx, "event-1"); err != nil || got.AmountCents != 1200 {
		t.Fatalf("settlement: %+v, %v", got, err)
	}
	if got, err := sim.PricingAndRewards(ctx, "member-1"); err != nil || got.RewardCents != 500 {
		t.Fatalf("pricing: %+v, %v", got, err)
	}
}

func assertCommandsAndFailures(t *testing.T, sim *Simulated, now time.Time) {
	t.Helper()
	ctx := context.Background()
	command := CommandRequest{ID: "command-1", DeviceID: "device-1", PowerKW: 1, ExpiresAt: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)}
	if got, err := sim.SendCommand(ctx, command); err != nil || !got.Accepted || got.ID != command.ID {
		t.Fatalf("command: %+v, %v", got, err)
	}
	if got, err := sim.SendCommand(ctx, command); err != nil || !got.Accepted {
		t.Fatalf("retry: %+v, %v", got, err)
	}
	command.PowerKW = 2
	if _, err := sim.SendCommand(ctx, command); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting retry: %v", err)
	}
	if _, err := sim.Market(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing market: %v", err)
	}
	if _, err := sim.SendCommand(ctx, CommandRequest{ID: "expired", DeviceID: "device-1", ExpiresAt: now}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expired command: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := sim.Market(canceled, "houston"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled market: %v", err)
	}
}

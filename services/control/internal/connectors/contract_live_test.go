package connectors

import (
	"context"
	"encoding/json"
)

type futureLiveFixture struct {
	rows map[string]map[string]string
}

func newFutureLiveFixture() *futureLiveFixture {
	return &futureLiveFixture{rows: map[string]map[string]string{
		"market":     {"houston": `{"Region":"houston","PriceUSDPerMWh":80,"LoadMW":1000,"At":"2026-08-12T18:00:00Z"}`},
		"weather":    {"houston": `{"Region":"houston","TemperatureC":34,"Alert":"heat","At":"2026-08-12T18:00:00Z"}`},
		"outage":     {"houston": `{"Region":"houston","Probability":0.2,"At":"2026-08-12T18:00:00Z"}`},
		"load":       {"site-1": `{"SiteID":"site-1","HomeKW":2,"SolarKW":1,"At":"2026-08-12T18:00:00Z"}`},
		"battery":    {"device-1": `{"DeviceID":"device-1","SOC":0.7,"PowerKW":1,"At":"2026-08-12T18:00:00Z"}`},
		"topology":   {"site-1": `{"SiteID":"site-1","FeederID":"feeder-1","TransformerID":""}`},
		"policy":     {"member-1": `{"MemberID":"member-1","Reserve":0.4,"Consented":true}`},
		"behavior":   {"member-1": `{"MemberID":"member-1","ResilienceTier":"BALANCED","TravelFlexActive":true}`},
		"settlement": {"event-1": `{"EventID":"event-1","AmountCents":1200}`},
		"pricing":    {"member-1": `{"MemberID":"member-1","CatalogVersion":"v1","RewardCents":500}`},
		"command":    {"command-1": `{"ID":"command-1","Accepted":true}`},
	}}
}

func decodeFixture[T any](ctx context.Context, rows map[string]string, key string) (T, error) {
	var value T
	if err := ctx.Err(); err != nil {
		return value, err
	}
	raw, ok := rows[key]
	if !ok {
		return value, ErrNotFound
	}
	err := json.Unmarshal([]byte(raw), &value)
	return value, err
}

func (l *futureLiveFixture) Market(ctx context.Context, key string) (MarketSnapshot, error) {
	return decodeFixture[MarketSnapshot](ctx, l.rows["market"], key)
}

func (l *futureLiveFixture) WeatherAndAlerts(ctx context.Context, key string) (WeatherSnapshot, error) {
	return decodeFixture[WeatherSnapshot](ctx, l.rows["weather"], key)
}

func (l *futureLiveFixture) OutageRisk(ctx context.Context, key string) (OutageSnapshot, error) {
	return decodeFixture[OutageSnapshot](ctx, l.rows["outage"], key)
}

func (l *futureLiveFixture) HouseholdLoad(ctx context.Context, key string) (LoadSnapshot, error) {
	return decodeFixture[LoadSnapshot](ctx, l.rows["load"], key)
}

func (l *futureLiveFixture) BatteryTelemetry(ctx context.Context, key string) (BatterySnapshot, error) {
	return decodeFixture[BatterySnapshot](ctx, l.rows["battery"], key)
}

func (l *futureLiveFixture) GridTopology(ctx context.Context, key string) (TopologySnapshot, error) {
	return decodeFixture[TopologySnapshot](ctx, l.rows["topology"], key)
}

func (l *futureLiveFixture) MemberPolicy(ctx context.Context, key string) (PolicySnapshot, error) {
	return decodeFixture[PolicySnapshot](ctx, l.rows["policy"], key)
}

func (l *futureLiveFixture) ResilienceAndTravelFlex(ctx context.Context, key string) (BehaviorSnapshot, error) {
	return decodeFixture[BehaviorSnapshot](ctx, l.rows["behavior"], key)
}

func (l *futureLiveFixture) Settlement(ctx context.Context, key string) (SettlementSnapshot, error) {
	return decodeFixture[SettlementSnapshot](ctx, l.rows["settlement"], key)
}

func (l *futureLiveFixture) PricingAndRewards(ctx context.Context, key string) (PricingSnapshot, error) {
	return decodeFixture[PricingSnapshot](ctx, l.rows["pricing"], key)
}

func (l *futureLiveFixture) SendCommand(ctx context.Context, request CommandRequest) (CommandReceipt, error) {
	return decodeFixture[CommandReceipt](ctx, l.rows["command"], request.ID)
}

var _ MarketPricesAndRegionalLoad = (*futureLiveFixture)(nil)
var _ WeatherAndAlerts = (*futureLiveFixture)(nil)
var _ OutageRisk = (*futureLiveFixture)(nil)
var _ HouseholdLoad = (*futureLiveFixture)(nil)
var _ BatteryTelemetry = (*futureLiveFixture)(nil)
var _ GridTopology = (*futureLiveFixture)(nil)
var _ MemberPolicy = (*futureLiveFixture)(nil)
var _ ResilienceAndTravelFlex = (*futureLiveFixture)(nil)
var _ Commands = (*futureLiveFixture)(nil)
var _ Settlement = (*futureLiveFixture)(nil)
var _ PricingAndRewards = (*futureLiveFixture)(nil)

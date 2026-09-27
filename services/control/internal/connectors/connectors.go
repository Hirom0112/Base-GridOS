package connectors

import (
	"context"
	"time"
)

type MarketSnapshot struct {
	Region         string
	PriceUSDPerMWh float64
	LoadMW         float64
	At             time.Time
}

type WeatherSnapshot struct {
	Region       string
	TemperatureC float64
	Alert        string
	At           time.Time
}

type OutageSnapshot struct {
	Region      string
	Probability float64
	At          time.Time
}

type LoadSnapshot struct {
	SiteID  string
	HomeKW  float64
	SolarKW float64
	At      time.Time
}

type BatterySnapshot struct {
	DeviceID string
	SOC      float64
	PowerKW  float64
	At       time.Time
}

type TopologySnapshot struct {
	SiteID        string
	FeederID      string
	TransformerID string
}

type PolicySnapshot struct {
	MemberID  string
	Reserve   float64
	Consented bool
}

type BehaviorSnapshot struct {
	MemberID         string
	ResilienceTier   string
	TravelFlexActive bool
}

type CommandRequest struct {
	ID        string
	DeviceID  string
	PowerKW   float64
	ExpiresAt time.Time
}

type CommandReceipt struct {
	ID       string
	Accepted bool
}

type SettlementSnapshot struct {
	EventID     string
	AmountCents int64
}

type PricingSnapshot struct {
	MemberID       string
	CatalogVersion string
	RewardCents    int64
}

type MarketPricesAndRegionalLoad interface {
	Market(context.Context, string) (MarketSnapshot, error)
}

type WeatherAndAlerts interface {
	WeatherAndAlerts(context.Context, string) (WeatherSnapshot, error)
}

type OutageRisk interface {
	OutageRisk(context.Context, string) (OutageSnapshot, error)
}

type HouseholdLoad interface {
	HouseholdLoad(context.Context, string) (LoadSnapshot, error)
}

type BatteryTelemetry interface {
	BatteryTelemetry(context.Context, string) (BatterySnapshot, error)
}

type GridTopology interface {
	GridTopology(context.Context, string) (TopologySnapshot, error)
}

type MemberPolicy interface {
	MemberPolicy(context.Context, string) (PolicySnapshot, error)
}

type ResilienceAndTravelFlex interface {
	ResilienceAndTravelFlex(context.Context, string) (BehaviorSnapshot, error)
}

type Commands interface {
	SendCommand(context.Context, CommandRequest) (CommandReceipt, error)
}

type Settlement interface {
	Settlement(context.Context, string) (SettlementSnapshot, error)
}

type PricingAndRewards interface {
	PricingAndRewards(context.Context, string) (PricingSnapshot, error)
}

const MarketLiveSlot = "PENDING-LIVE"
const WeatherLiveSlot = "PENDING-LIVE"
const OutageLiveSlot = "PENDING-LIVE"
const HouseholdLoadLiveSlot = "PENDING-LIVE"
const BatteryLiveSlot = "PENDING-LIVE"
const TopologyLiveSlot = "PENDING-LIVE"
const MemberPolicyLiveSlot = "PENDING-LIVE"
const BehaviorLiveSlot = "PENDING-LIVE"
const CommandsLiveSlot = "PENDING-LIVE"
const SettlementLiveSlot = "PENDING-LIVE"
const PricingLiveSlot = "PENDING-LIVE"

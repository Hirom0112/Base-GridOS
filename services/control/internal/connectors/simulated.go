package connectors

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("connector record not found")
var ErrInvalidRequest = errors.New("invalid connector request")
var ErrConflict = errors.New("idempotency key conflicts with prior command")

type Data struct {
	Markets     map[string]MarketSnapshot
	Weather     map[string]WeatherSnapshot
	Outages     map[string]OutageSnapshot
	Loads       map[string]LoadSnapshot
	Batteries   map[string]BatterySnapshot
	Topology    map[string]TopologySnapshot
	Policies    map[string]PolicySnapshot
	Behavior    map[string]BehaviorSnapshot
	Settlements map[string]SettlementSnapshot
	Pricing     map[string]PricingSnapshot
}

type Simulated struct {
	data     Data
	mu       sync.Mutex
	commands map[string]CommandRequest
}

func NewSimulated(data Data) *Simulated {
	return &Simulated{data: data, commands: make(map[string]CommandRequest)}
}

func lookup[T any](ctx context.Context, key string, records map[string]T) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if key == "" {
		return zero, ErrInvalidRequest
	}
	value, ok := records[key]
	if !ok {
		return zero, ErrNotFound
	}
	return value, nil
}

func (s *Simulated) Market(ctx context.Context, region string) (MarketSnapshot, error) {
	return lookup(ctx, region, s.data.Markets)
}

func (s *Simulated) WeatherAndAlerts(ctx context.Context, region string) (WeatherSnapshot, error) {
	return lookup(ctx, region, s.data.Weather)
}

func (s *Simulated) OutageRisk(ctx context.Context, region string) (OutageSnapshot, error) {
	return lookup(ctx, region, s.data.Outages)
}

func (s *Simulated) HouseholdLoad(ctx context.Context, siteID string) (LoadSnapshot, error) {
	return lookup(ctx, siteID, s.data.Loads)
}

func (s *Simulated) BatteryTelemetry(ctx context.Context, deviceID string) (BatterySnapshot, error) {
	return lookup(ctx, deviceID, s.data.Batteries)
}

func (s *Simulated) GridTopology(ctx context.Context, siteID string) (TopologySnapshot, error) {
	return lookup(ctx, siteID, s.data.Topology)
}

func (s *Simulated) MemberPolicy(ctx context.Context, memberID string) (PolicySnapshot, error) {
	return lookup(ctx, memberID, s.data.Policies)
}

func (s *Simulated) ResilienceAndTravelFlex(ctx context.Context, memberID string) (BehaviorSnapshot, error) {
	return lookup(ctx, memberID, s.data.Behavior)
}

func (s *Simulated) Settlement(ctx context.Context, eventID string) (SettlementSnapshot, error) {
	return lookup(ctx, eventID, s.data.Settlements)
}

func (s *Simulated) PricingAndRewards(ctx context.Context, memberID string) (PricingSnapshot, error) {
	return lookup(ctx, memberID, s.data.Pricing)
}

func (s *Simulated) SendCommand(ctx context.Context, request CommandRequest) (CommandReceipt, error) {
	if err := ctx.Err(); err != nil {
		return CommandReceipt{}, err
	}
	if request.ID == "" || request.DeviceID == "" || !request.ExpiresAt.After(time.Now()) {
		return CommandReceipt{}, ErrInvalidRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if prior, ok := s.commands[request.ID]; ok {
		if prior != request {
			return CommandReceipt{}, ErrConflict
		}
		return CommandReceipt{ID: request.ID, Accepted: true}, nil
	}
	s.commands[request.ID] = request
	return CommandReceipt{ID: request.ID, Accepted: true}, nil
}

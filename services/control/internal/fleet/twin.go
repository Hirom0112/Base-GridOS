package fleet

import (
	"sync"
	"time"
)

type OperatingState string

const (
	OnGrid                    OperatingState = "ON_GRID"
	OffGridOutage             OperatingState = "OFF_GRID_OUTAGE"
	OffGridNoHomePower        OperatingState = "OFF_GRID_NO_HOME_POWER"
	OffGridOvercurrent        OperatingState = "OFF_GRID_OVERCURRENT"
	OffGridOvercurrentStandby OperatingState = "OFF_GRID_OVERCURRENT_STANDBY"
	TelemetryUnavailable      OperatingState = "TELEMETRY_UNAVAILABLE"
)

type Availability string

const (
	Online      Availability = "ONLINE"
	Offline     Availability = "OFFLINE"
	Degraded    Availability = "DEGRADED"
	Stale       Availability = "STALE"
	Maintenance Availability = "MAINTENANCE"
)

type SiteState struct {
	SiteID             string
	ObservedAt         time.Time
	OperatingState     OperatingState
	Availability       Availability
	DispatchableKW     float64
	DispatchableKWh    float64
	EnergyKWh          float64
	ReserveKWh         float64
	HardwareFloorKWh   float64
	BackupHoursCurrent float64
	BackupHours750W    float64
	Provenance         string
}

type Quantity struct {
	Value         float64
	Timestamp     time.Time
	ProvenanceMix map[string]int
	Freshness     time.Duration
}

type Aggregate struct {
	DispatchableMW     Quantity
	DispatchableMWh    Quantity
	BackupHoursCurrent float64
	BackupHours750W    float64
}

type Twin struct {
	mu                 sync.RWMutex
	freshnessThreshold time.Duration
	sites              map[string]SiteState
}

func NewTwin(freshnessThreshold time.Duration) *Twin {
	return &Twin{freshnessThreshold: freshnessThreshold, sites: make(map[string]SiteState)}
}

func (t *Twin) Accept(state SiteState) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	current, found := t.sites[state.SiteID]
	if found && !state.ObservedAt.After(current.ObservedAt) {
		return false
	}
	t.sites[state.SiteID] = state
	return true
}

func (t *Twin) CommandIssued(string, float64) {
}

func (t *Twin) Site(siteID string, now time.Time) (SiteState, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	state, found := t.sites[siteID]
	if found && now.Sub(state.ObservedAt) > t.freshnessThreshold {
		state.Availability = Stale
	}
	return state, found
}

func (t *Twin) Sites(now time.Time) []SiteState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	states := make([]SiteState, 0, len(t.sites))
	for _, state := range t.sites {
		if now.Sub(state.ObservedAt) > t.freshnessThreshold {
			state.Availability = Stale
		} else if state.Availability == "" {
			state.Availability = Online
		}
		states = append(states, state)
	}
	return states
}

func (t *Twin) Aggregate(now time.Time) Aggregate {
	t.mu.RLock()
	defer t.mu.RUnlock()
	provenance := make(map[string]int)
	var powerKW, energyKWh, backupCurrent, backup750 float64
	var freshness time.Duration
	for _, state := range t.sites {
		age := max(now.Sub(state.ObservedAt), 0)
		if age > t.freshnessThreshold || state.OperatingState != OnGrid {
			continue
		}
		powerKW += state.DispatchableKW
		energyKWh += state.DispatchableKWh
		backupCurrent += state.BackupHoursCurrent
		backup750 += state.BackupHours750W
		provenance[state.Provenance]++
		freshness = max(freshness, age)
	}
	metadata := func(value float64) Quantity {
		return Quantity{Value: value, Timestamp: now, ProvenanceMix: provenance, Freshness: freshness}
	}
	return Aggregate{
		DispatchableMW:     metadata(powerKW / 1000),
		DispatchableMWh:    metadata(energyKWh / 1000),
		BackupHoursCurrent: backupCurrent,
		BackupHours750W:    backup750,
	}
}

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

type RiskPolicy struct {
	Version                    string
	EffectiveAt                time.Time
	ExpiresAt                  time.Time
	OutageProbabilityThreshold float64
	TelemetryFreshness         time.Duration
	GatewayCadence             time.Duration
	WeatherFloor               float64
	OutageFloor                float64
	StaleFloor                 float64
	AlarmFloor                 float64
	CommunicationsFloor        float64
	HealthFloor                float64
	Provenance                 string
	WeatherZoneUGC             map[string]map[string]WeatherAreaCodes
}

type WeatherAreaCodes struct {
	UGC  []string `json:"ugc"`
	SAME []string `json:"same"`
}

type RiskWeather struct {
	EvidenceID string
	AsOf       time.Time
	Expires    time.Time
	Active     bool
	Provenance string
}

type RiskOutage struct {
	EvidenceID        string
	AsOf              time.Time
	HourlyProbability float64
}

type RiskTelemetry struct {
	EvidenceID string
	ObservedAt time.Time
	Alarm      bool
}

type RiskGateway struct {
	EvidenceID      string
	LastPublishedAt time.Time
}

type RiskSignals struct {
	At        time.Time
	Weather   *RiskWeather
	Outage    *RiskOutage
	Telemetry *RiskTelemetry
	Gateway   *RiskGateway
}

type RiskDecision struct {
	Reason       OverrideReason
	FloorPercent float64
	EvidenceID   string
	AsOf         time.Time
	ExpiresAt    time.Time
}

func (store *Store) RiskPolicyAt(ctx context.Context, at time.Time) (RiskPolicy, error) {
	var value RiskPolicy
	var freshness, cadence float64
	var encoded []byte
	err := store.pool.QueryRow(ctx, `SELECT version,effective_at,expires_at,outage_probability_threshold,
		telemetry_freshness_seconds,gateway_cadence_seconds,weather_floor_percent,outage_floor_percent,
		stale_floor_percent,alarm_floor_percent,communications_floor_percent,
		health_floor_percent,weather_zone_ugc,provenance->>'provenance'
		FROM risk_policy WHERE effective_at <= $1 AND expires_at > $1`, at).Scan(
		&value.Version, &value.EffectiveAt, &value.ExpiresAt, &value.OutageProbabilityThreshold,
		&freshness, &cadence, &value.WeatherFloor, &value.OutageFloor, &value.StaleFloor,
		&value.AlarmFloor, &value.CommunicationsFloor, &value.HealthFloor, &encoded, &value.Provenance)
	if errors.Is(err, pgx.ErrNoRows) {
		return RiskPolicy{}, errors.New("risk policy is not effective")
	}
	if err != nil {
		return RiskPolicy{}, err
	}
	if err := json.Unmarshal(encoded, &value.WeatherZoneUGC); err != nil {
		return RiskPolicy{}, err
	}
	value.TelemetryFreshness = time.Duration(freshness * float64(time.Second))
	value.GatewayCadence = time.Duration(cadence * float64(time.Second))
	return value, nil
}

func (policy RiskPolicy) Evaluate(signals RiskSignals) []RiskDecision {
	var decisions []RiskDecision
	cycleEnd := signals.At.Add(5 * time.Minute)
	add := func(reason OverrideReason, floor float64, id string, asOf, expiresAt time.Time) {
		if id != "" && !asOf.IsZero() && !asOf.After(signals.At) {
			decisions = append(decisions, RiskDecision{Reason: reason, FloorPercent: floor, EvidenceID: id, AsOf: asOf, ExpiresAt: expiresAt})
		}
	}
	if weather := signals.Weather; weather != nil && weather.Active {
		add(OverrideWeather, policy.WeatherFloor, weather.EvidenceID, weather.AsOf, weather.Expires)
	}
	if outage := signals.Outage; outage != nil && !math.IsNaN(outage.HourlyProbability) && !math.IsInf(outage.HourlyProbability, 0) && outage.HourlyProbability > policy.OutageProbabilityThreshold && outage.HourlyProbability <= 1 {
		add(OverrideOutageRisk, policy.OutageFloor, outage.EvidenceID, outage.AsOf, cycleEnd)
	}
	if telemetry := signals.Telemetry; telemetry != nil {
		if signals.At.Sub(telemetry.ObservedAt) > policy.TelemetryFreshness {
			add(OverrideStaleTelemetry, policy.StaleFloor, telemetry.EvidenceID, telemetry.ObservedAt, cycleEnd)
		}
		if telemetry.Alarm {
			add(OverrideAlarm, policy.AlarmFloor, telemetry.EvidenceID, telemetry.ObservedAt, cycleEnd)
		}
	}
	if gateway := signals.Gateway; gateway != nil && signals.At.Sub(gateway.LastPublishedAt) > 2*policy.GatewayCadence {
		add(OverrideCommunications, policy.CommunicationsFloor, gateway.EvidenceID, gateway.LastPublishedAt, cycleEnd)
	}
	return decisions
}

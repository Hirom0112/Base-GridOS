package policy

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRiskBridgeUsesVersionedThresholdsAndEvidence(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `INSERT INTO risk_policy
		(version,effective_at,expires_at,outage_probability_threshold,telemetry_freshness_seconds,
		gateway_cadence_seconds,weather_floor_percent,outage_floor_percent,stale_floor_percent,
		alarm_floor_percent,communications_floor_percent)
		VALUES ('risk-v1',$1,$2,0.01,30,5,85,82,80,95,88)`, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)
	selected, err := New(pool).RiskPolicyAt(ctx, now)
	require.NoError(t, err)
	require.Equal(t, "risk-v1", selected.Version)
	decisions := selected.Evaluate(RiskSignals{At: now,
		Weather:   &RiskWeather{EvidenceID: "nws-1", AsOf: now.Add(-time.Minute), Active: true},
		Outage:    &RiskOutage{EvidenceID: "outage-1", AsOf: now.Add(-time.Minute), HourlyProbability: 0.02},
		Telemetry: &RiskTelemetry{EvidenceID: "observation-1", ObservedAt: now.Add(-time.Minute), Alarm: true},
		Gateway:   &RiskGateway{EvidenceID: "gateway-1", LastPublishedAt: now.Add(-11 * time.Second)},
	})
	require.Len(t, decisions, 5)
	require.Equal(t, OverrideWeather, decisions[0].Reason)
	require.Equal(t, "nws-1", decisions[0].EvidenceID)
	require.Equal(t, 85.0, decisions[0].FloorPercent)
	require.Equal(t, OverrideCommunications, decisions[4].Reason)
	require.Equal(t, "gateway-1", decisions[4].EvidenceID)

	quiet := selected.Evaluate(RiskSignals{At: now,
		Outage:    &RiskOutage{EvidenceID: "outage-low", AsOf: now, HourlyProbability: 0.01},
		Telemetry: &RiskTelemetry{EvidenceID: "fresh", ObservedAt: now.Add(-10 * time.Second)},
		Gateway:   &RiskGateway{EvidenceID: "recent", LastPublishedAt: now.Add(-10 * time.Second)},
	})
	require.Empty(t, quiet)
}

func TestRiskBridgeMigrationReapplies(t *testing.T) {
	pool := policyDatabase(t)
	forward, err := os.ReadFile("../../../../../database/migrations/0014_risk_policy.sql")
	require.NoError(t, err)
	rollback, err := os.ReadFile("../../../../../database/rollback/0014_risk_policy.sql")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(forward))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(rollback))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(forward))
	require.NoError(t, err)
}

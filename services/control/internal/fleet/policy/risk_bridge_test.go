package policy

import (
	"context"
	"os"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	publiccontext "github.com/Hirom0112/Base-GridOS/services/control/internal/context"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestRiskBridgeUsesVersionedThresholdsAndEvidence(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `INSERT INTO risk_policy
		(version,effective_at,expires_at,outage_probability_threshold,telemetry_freshness_seconds,
		gateway_cadence_seconds,weather_floor_percent,outage_floor_percent,stale_floor_percent,
		alarm_floor_percent,communications_floor_percent,health_floor_percent,weather_zone_ugc,provenance)
		VALUES ('risk-v1',$1,$2,0.01,30,5,85,82,80,95,88,100,'{}','{"provenance":"SIMULATED"}')`, now.Add(-time.Hour), now.Add(time.Hour))
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

func TestRiskBridgeAppliesStaleOnlyWithConsentedPlan(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	seedPolicyCatalog(t, pool, now.Add(-48*time.Hour))
	_, err := pool.Exec(ctx, `INSERT INTO risk_policy(version,effective_at,expires_at,outage_probability_threshold,
		telemetry_freshness_seconds,gateway_cadence_seconds,weather_floor_percent,outage_floor_percent,
		stale_floor_percent,alarm_floor_percent,communications_floor_percent,health_floor_percent,weather_zone_ugc,provenance)
		VALUES ('risk-test',$1,$2,0.01,30,15,60,60,40,100,40,100,'{}','{"provenance":"SIMULATED"}')`, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO member_sites(site_id,member_id,bound_at,source,provenance)
		VALUES ('site-consented','member-consented',$1,'SIMULATED','{"provenance":"SIMULATED"}'),
		('site-no-plan','member-no-plan',$1,'SIMULATED','{"provenance":"SIMULATED"}')`, now.Add(-time.Hour))
	require.NoError(t, err)
	_, err = selectWithOffer(t, New(pool), Selection{ID: "risk-selection", MemberID: "member-consented",
		Market: "TX", CatalogVersion: "catalog-v2", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v2",
		ConsentText: "I consent", ConsentVersion: "v1", ExplanationShown: "Backup reserve",
		EffectiveAt: now.Add(-time.Minute), CorrelationID: "risk-selection"})
	require.NoError(t, err)
	telemetry := storage.NewTelemetryStoreAt(pool, func() time.Time { return now })
	sites := []*gridosv1.AuthorizedSite{
		{Site: &gridosv1.Site{SiteId: "site-consented", WeatherZone: "SCENT"}, Devices: []*gridosv1.Device{{DeviceId: "device-consented"}}},
		{Site: &gridosv1.Site{SiteId: "site-no-plan", WeatherZone: "SCENT"}, Devices: []*gridosv1.Device{{DeviceId: "device-no-plan"}}},
	}
	for _, deviceID := range []string{"device-consented", "device-no-plan"} {
		_, err = telemetry.Write(ctx, "gateway-1", []*gridosv1.TelemetryObservation{{ObservationId: deviceID + ":observation",
			DeviceId: deviceID, Sequence: 1, ObservationTime: timestamppb.New(now.Add(-time.Minute)),
			OperatingState: &gridosv1.TelemetryObservation_OnGrid{OnGrid: &gridosv1.OnGrid{ObservedAt: timestamppb.New(now.Add(-time.Minute))}}}})
		require.NoError(t, err)
	}
	require.NoError(t, NewRiskBridge(pool, sites, "", "risk-test.jsonl").Evaluate(ctx, now))
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reserve_overrides WHERE member_id = 'member-consented' AND reason = 'STALE_TELEMETRY' AND reserve_floor_percent = 40`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reserve_overrides WHERE member_id = 'member-no-plan'`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM risk_policy_evaluations WHERE evaluated_at = $1`, now).Scan(&count))
	require.Equal(t, 2, count)
}

func TestRiskBridgeWeatherMatchesOnlyMappedFleetAndZone(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	policy := RiskPolicy{WeatherFloor: 60, WeatherZoneUGC: map[string]map[string]WeatherAreaCodes{
		"austin-5000.jsonl": {"SCENT": {UGC: []string{"TXZ192"}, SAME: []string{"048453"}}},
	}}
	site := &gridosv1.AuthorizedSite{Site: &gridosv1.Site{SiteId: "austin-site", WeatherZone: "SCENT"}}
	alert := publiccontext.Alert{ID: "nws-travis-1", UGC: []string{"TXZ192"}, SAME: []string{"048453"},
		Effective: now.Add(-time.Minute), Expires: now.Add(time.Hour), Source: publiccontext.Source{AsOf: now.Add(-time.Minute)}}
	source := riskSource{public: publiccontext.Snapshot{Alerts: []publiccontext.Alert{alert}}}
	evidence, decisions := riskForSite(now, policy, source, site, "testdata/fleets/austin-5000.jsonl")
	require.Len(t, decisions, 1)
	require.Equal(t, OverrideWeather, decisions[0].Reason)
	require.Equal(t, "nws-travis-1", evidence.Weather.EvidenceID)
	evidence, decisions = riskForSite(now, policy, source, site, "testdata/fleets/texas-50.jsonl")
	require.Empty(t, decisions)
	require.Contains(t, evidence.Missing, "weather_source_unmatched")
	evidence, decisions = riskForSite(now, policy, riskSource{}, site, "testdata/fleets/austin-5000.jsonl")
	require.Empty(t, decisions)
	require.Contains(t, evidence.Missing, "weather_no_active_alert")
}

func TestRiskBridgeAwayAnomalyUsesLatestMeasuredHomeLoad(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id,member_id,bound_at,source,provenance)
		VALUES ('away-site','away-member',$1,'SIMULATED','{"provenance":"SIMULATED"}')`, now.Add(-time.Hour))
	require.NoError(t, err)
	store := New(pool)
	require.NoError(t, store.SetAnomalyPreference(ctx, AnomalyPreference{ID: "away-preference", MemberID: "away-member",
		OptIn: true, ConsentText: "notify on home load", ConsentVersion: "v1", BaselineUpperKW: 1,
		BaselineBegin: now.Add(-time.Hour), BaselineEnd: now.Add(time.Hour),
		EffectiveAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), CorrelationID: "away-preference"}))
	require.NoError(t, store.ScheduleAway(ctx, AwayPeriod{ID: "away-period", MemberID: "away-member",
		Start: now.Add(-time.Hour), End: now.Add(time.Hour), ConsentVersion: "v1", CorrelationID: "away-period"}))
	sites := []*gridosv1.AuthorizedSite{{Site: &gridosv1.Site{SiteId: "away-site"},
		Devices: []*gridosv1.Device{{DeviceId: "away-device"}}}}
	telemetry := storage.NewTelemetryStoreAt(pool, func() time.Time { return now })
	_, err = telemetry.Write(ctx, "away-gateway", []*gridosv1.TelemetryObservation{{ObservationId: "away-observation",
		DeviceId: "away-device", Sequence: 1, ObservationTime: timestamppb.New(now),
		ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT, PowerFlow: &gridosv1.PowerFlow{ToHomeKw: 3}}})
	require.NoError(t, err)
	bridge := NewRiskBridge(pool, sites, "", "away-fleet.jsonl")
	require.NoError(t, bridge.EvaluateAnomalies(ctx, now))
	require.NoError(t, bridge.EvaluateAnomalies(ctx, now))
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM member_alerts WHERE member_id = 'away-member' AND message = 'energy anomaly signal'`).Scan(&count))
	require.Equal(t, 1, count)
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

package policy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAnomalyNeedsOptInAwayAndMeasuredLoadAboveConsentedBound(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id, member_id, bound_at, source, provenance)
		VALUES ('site-anomaly', 'member-anomaly', $1, 'SIMULATED', '{"provenance":"SIMULATED"}')`, begin)
	require.NoError(t, err)
	store := New(pool)
	preference := AnomalyPreference{ID: "preference-anomaly", MemberID: "member-anomaly", OptIn: true, ConsentText: "I opt in to energy anomaly signals", ConsentVersion: "anomaly-v1", BaselineUpperKW: 1, BaselineBegin: begin, BaselineEnd: begin.Add(2 * time.Hour), EffectiveAt: begin, ExpiresAt: begin.Add(24 * time.Hour), CorrelationID: "preference-anomaly"}
	require.NoError(t, store.SetAnomalyPreference(ctx, preference))
	reading := MeasuredSiteLoad{ObservationID: "observation-high", SiteID: "site-anomaly", ObservedAt: begin.Add(time.Minute), ToHomeKW: 2, CorrelationID: "observation-high"}
	beforeAway, err := store.EvaluateAnomaly(ctx, reading)
	require.NoError(t, err)
	require.Nil(t, beforeAway)
	away := AwayPeriod{ID: "away-anomaly", MemberID: "member-anomaly", Start: begin, End: begin.Add(time.Hour), ConsentVersion: "away-v1", CorrelationID: "away-anomaly"}
	require.NoError(t, store.ScheduleAway(ctx, away))
	require.NoError(t, store.ScheduleAway(ctx, away))
	reading.ToHomeKW = 0.8
	normal, err := store.EvaluateAnomaly(ctx, reading)
	require.NoError(t, err)
	require.Nil(t, normal)
	reading.ToHomeKW = 2
	alert, err := store.EvaluateAnomaly(ctx, reading)
	require.NoError(t, err)
	require.NotNil(t, alert)
	require.Equal(t, "energy anomaly signal", alert.Text)
	require.Equal(t, "member-anomaly", alert.MemberID)
	retried, err := store.EvaluateAnomaly(ctx, reading)
	require.NoError(t, err)
	require.Equal(t, alert, retried)
	var alerts, audits int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM member_alerts WHERE alert_id = $1`, alert.ID).Scan(&alerts))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_journal WHERE action = 'ENERGY_ANOMALY_SIGNAL' AND resource_id = $1`, alert.ID).Scan(&audits))
	require.Equal(t, 1, alerts)
	require.Equal(t, 1, audits)
	require.NoError(t, store.EndAway(ctx, EndAway{ID: "end-away", PeriodID: away.ID, MemberID: away.MemberID, At: begin.Add(30 * time.Minute), CorrelationID: "end-away"}))
	reading.ObservationID = "observation-after-return"
	reading.ObservedAt = begin.Add(31 * time.Minute)
	afterReturn, err := store.EvaluateAnomaly(ctx, reading)
	require.NoError(t, err)
	require.Nil(t, afterReturn)
}

func TestAnomalyWithoutOptInDoesNotAlertDuringAway(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id, member_id, bound_at, source, provenance)
		VALUES ('site-optout', 'member-optout', $1, 'SIMULATED', '{"provenance":"SIMULATED"}')`, begin)
	require.NoError(t, err)
	store := New(pool)
	require.NoError(t, store.ScheduleAway(ctx, AwayPeriod{ID: "away-optout", MemberID: "member-optout", Start: begin, End: begin.Add(time.Hour), ConsentVersion: "away-v1", CorrelationID: "away-optout"}))
	reading := MeasuredSiteLoad{ObservationID: "observation-optout", SiteID: "site-optout", ObservedAt: begin.Add(time.Minute), ToHomeKW: 5, CorrelationID: "observation-optout"}
	alert, err := store.EvaluateAnomaly(ctx, reading)
	require.NoError(t, err)
	require.Nil(t, alert)
	require.NoError(t, store.SetAnomalyPreference(ctx, AnomalyPreference{ID: "preference-optout", MemberID: "member-optout", OptIn: false, ConsentText: "I decline anomaly alerts", ConsentVersion: "anomaly-v1", BaselineUpperKW: 1, BaselineBegin: begin, BaselineEnd: begin.Add(time.Hour), EffectiveAt: begin, ExpiresAt: begin.Add(24 * time.Hour), CorrelationID: "preference-optout"}))
	alert, err = store.EvaluateAnomaly(ctx, reading)
	require.NoError(t, err)
	require.Nil(t, alert)
}

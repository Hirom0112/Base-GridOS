package policy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOverrideRaisesReserveImmediatelyAndExpires(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	store := New(pool)
	_, err := store.Select(ctx, Selection{ID: "selection-override", MemberID: "member-override", Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1", ConsentText: "I consent", ConsentVersion: "v1", ExplanationShown: "Backup reserve", EffectiveAt: begin.Add(-time.Hour), CorrelationID: "selection-override"})
	require.NoError(t, err)
	expires := begin.Add(time.Hour)
	command := ReserveOverride{ID: "override-weather", MemberID: "member-override", Reason: "WEATHER", FloorPercent: 85, EffectiveAt: begin, ExpiresAt: expires, PolicyVersion: "policy-v1", EvidenceID: "nws-alert-1", CorrelationID: "override-weather"}
	before, err := store.EffectiveReserve(ctx, command.MemberID, begin.Add(-time.Nanosecond))
	require.NoError(t, err)
	require.Equal(t, 65.0, before)
	require.NoError(t, store.ApplyOverride(ctx, command))
	require.NoError(t, store.ApplyOverride(ctx, command))
	inside, err := store.EffectiveReserve(ctx, command.MemberID, begin)
	require.NoError(t, err)
	require.Equal(t, 85.0, inside)
	after, err := store.EffectiveReserve(ctx, command.MemberID, expires)
	require.NoError(t, err)
	require.Equal(t, 65.0, after)
	var audits int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_journal WHERE action = 'RESERVE_OVERRIDE_APPLIED' AND resource_id = $1 AND new_values->>'evidence_id' = $2`, command.ID, command.EvidenceID).Scan(&audits))
	require.Equal(t, 1, audits)
	command.FloorPercent = 90
	require.Error(t, store.ApplyOverride(ctx, command))
}

func TestOverrideRequiresRaisedReserveAndSupportedSignal(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	store := New(pool)
	_, err := store.Select(ctx, Selection{ID: "selection-risk", MemberID: "member-risk", Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1", ConsentText: "I consent", ConsentVersion: "v1", ExplanationShown: "Backup reserve", EffectiveAt: begin.Add(-time.Hour), CorrelationID: "selection-risk"})
	require.NoError(t, err)
	command := ReserveOverride{ID: "override-risk", MemberID: "member-risk", Reason: "COMMUNICATIONS", FloorPercent: 80, EffectiveAt: begin, ExpiresAt: begin.Add(time.Hour), PolicyVersion: "policy-v1", EvidenceID: "gateway-gap-1", CorrelationID: "override-risk"}
	require.NoError(t, store.ApplyOverride(ctx, command))
	reserve, err := store.EffectiveReserve(ctx, command.MemberID, begin)
	require.NoError(t, err)
	require.Equal(t, 80.0, reserve)
	command.ID = "too-low"
	command.FloorPercent = 60
	require.Error(t, store.ApplyOverride(ctx, command))
	command.FloorPercent = 90
	command.Reason = "UNKNOWN"
	require.Error(t, store.ApplyOverride(ctx, command))
	command.Reason = "ALARM"
	command.EvidenceID = ""
	require.Error(t, store.ApplyOverride(ctx, command))
}

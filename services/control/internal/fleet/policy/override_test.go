package policy

import (
	"context"
	"os"
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
	command.FloorPercent = 85
	command.EvidenceID = "different-alert"
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

func TestOverridePersistsAcrossPlanChangeUntilExpiry(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	store := New(pool)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id, member_id, bound_at, source, provenance)
		VALUES ('site-override', 'member-override', $1, 'SIMULATED', '{"provenance":"SIMULATED"}')`, begin)
	require.NoError(t, err)
	_, err = store.Select(ctx, Selection{ID: "selection-original", MemberID: "member-override", Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1", ConsentText: "I consent", ConsentVersion: "v1", ExplanationShown: "Backup reserve", EffectiveAt: begin, CorrelationID: "selection-original"})
	require.NoError(t, err)
	expires := begin.Add(48 * time.Hour)
	require.NoError(t, store.ApplyOverride(ctx, ReserveOverride{ID: "override-lasting", MemberID: "member-override", Reason: OverrideWeather, FloorPercent: 85, EffectiveAt: begin, ExpiresAt: expires, PolicyVersion: "policy-v1", EvidenceID: "nws-alert-lasting", CorrelationID: "override-lasting"}))
	changedAt := begin.Add(25 * time.Hour)
	_, err = store.Select(ctx, Selection{ID: "selection-new", MemberID: "member-override", Market: "TX", CatalogVersion: "catalog-v2", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v2", ConsentText: "I consent to new plan", ConsentVersion: "v2", ExplanationShown: "New backup reserve", EffectiveAt: changedAt, CorrelationID: "selection-new"})
	require.NoError(t, err)
	reserve, err := store.ReserveAt(ctx, "member-override", changedAt)
	require.NoError(t, err)
	require.Equal(t, 85.0, reserve.EffectivePercent)
	bySite, err := store.SiteReserves(ctx, []string{"site-override"}, changedAt)
	require.NoError(t, err)
	require.Equal(t, 85.0, bySite["site-override"].BasePercent)
}

func TestOverrideMigrationReappliesAfterRollback(t *testing.T) {
	pool := policyDatabase(t)
	forward, err := os.ReadFile("../../../../../database/migrations/0011_reserve_override_signals.sql")
	require.NoError(t, err)
	rollback, err := os.ReadFile("../../../../../database/rollback/0011_reserve_override_signals.sql")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(forward))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(rollback))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(forward))
	require.NoError(t, err)
	var present bool
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid = 'reserve_overrides'::regclass AND attname = 'evidence_id' AND NOT attisdropped)`).Scan(&present))
	require.True(t, present)
}

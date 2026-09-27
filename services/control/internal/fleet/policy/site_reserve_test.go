package policy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTravelFlexSiteReservesRequireBindingPlanAndActiveConsent(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id, member_id, bound_at, source, provenance) VALUES
		('site-flex', 'member-flex', $1, 'SIMULATED', '{"provenance":"SIMULATED"}'),
		('site-no-plan', 'member-no-plan', $1, 'SIMULATED', '{"provenance":"SIMULATED"}')`, begin)
	require.NoError(t, err)
	store := New(pool)
	_, err = selectWithOffer(t, store, Selection{ID: "selection-flex", MemberID: "member-flex", Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1", ConsentText: "I consent", ConsentVersion: "v1", ExplanationShown: "Backup reserve", EffectiveAt: begin, CorrelationID: "selection-flex"})
	require.NoError(t, err)
	start := begin.Add(time.Hour)
	_, err = scheduleWithOffer(t, store, TravelFlex{ID: "window-flex", MemberID: "member-flex", Start: start, End: start.Add(time.Hour), Timezone: "UTC", TemporaryReservePercent: 20, EarlyReturnAction: RestorePlanReserve, CreditType: "FIXED_EVENT", CreditCents: 500, ConsentText: "I consent to the credit", ConsentVersion: "v1", PolicyVersion: "policy-v1", CorrelationID: "window-flex"})
	require.NoError(t, err)
	sites := []string{"site-flex", "site-no-plan", "site-unbound"}
	before, err := store.SiteReserves(ctx, sites, start.Add(-time.Nanosecond))
	require.NoError(t, err)
	require.Equal(t, 65.0, before["site-flex"].BasePercent)
	require.Nil(t, before["site-flex"].TravelFlexPercent)
	require.NotContains(t, before, "site-no-plan")
	require.NotContains(t, before, "site-unbound")
	inside, err := store.SiteReserves(ctx, sites, start)
	require.NoError(t, err)
	require.Equal(t, 65.0, inside["site-flex"].BasePercent)
	require.NotNil(t, inside["site-flex"].TravelFlexPercent)
	require.Equal(t, 20.0, *inside["site-flex"].TravelFlexPercent)
	after, err := store.SiteReserves(ctx, sites, start.Add(time.Hour))
	require.NoError(t, err)
	require.Nil(t, after["site-flex"].TravelFlexPercent)
	crossing, err := store.SiteReservesForWindow(ctx, sites, start, start.Add(2*time.Hour))
	require.NoError(t, err)
	require.Nil(t, crossing["site-flex"].TravelFlexPercent)
	contained, err := store.SiteReservesForWindow(ctx, sites, start, start.Add(30*time.Minute))
	require.NoError(t, err)
	require.NotNil(t, contained["site-flex"].TravelFlexPercent)
}

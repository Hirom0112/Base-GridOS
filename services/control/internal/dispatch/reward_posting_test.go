package dispatch

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/policy"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestRewardPostingActivityCommitsCreditBeforeReportAndRetriesOnce(t *testing.T) {
	harness := newActivityHarness(t)
	ctx := context.Background()
	begin := harness.input.Request.GetBeginTime().AsTime()
	snapshotter := harness.activities.Dispatcher.Snapshots.(activitySnapshotter)
	snapshotter.snapshot.Optimization.Devices = []*gridosv1.DeviceState{{DeviceId: "device-1", SiteId: "site-1"}}
	harness.activities.Dispatcher.Snapshots = snapshotter
	_, err := harness.pool.Exec(ctx, `INSERT INTO member_sites(site_id,member_id,bound_at,source,provenance)
		VALUES ('site-1','member-1',$1,'SIMULATED','{"provenance":"SIMULATED"}')`, begin.Add(-time.Hour))
	require.NoError(t, err)
	_, err = harness.pool.Exec(ctx, `INSERT INTO reserve_policies(policy_version,protected_hardware_floor_percent,
		member_plan_floor_percent,dynamic_override_percent,effective_reserve_percent,effective_at,correlation_id,provenance)
		VALUES ('reward-policy',10,10,0,10,$1,'reward-test','{"provenance":"SIMULATED"}')`, begin.Add(-time.Hour))
	require.NoError(t, err)
	_, err = harness.pool.Exec(ctx, `INSERT INTO pricing_catalog_snapshots(catalog_version,member_plan_id,market,
		display_name,reserve_floor_percent,energy_plan,energy_term_months,energy_monthly_charge_cents,
		battery_plan,battery_term_months,battery_monthly_charge_cents,flexibility_reward_cents,effective_at,correlation_id,provenance)
		VALUES ('reward-catalog','essential','ERCOT','Essential',10,'{}',0,0,'{}',0,0,500,$1,'reward-test','{"provenance":"SIMULATED"}')`, begin.Add(-time.Hour))
	require.NoError(t, err)
	store := policy.New(harness.pool)
	_, err = store.PresentOffer(ctx, policy.Offer{ID: "reward-offer", MemberID: "member-1", Kind: policy.PlanOffer,
		Market: "ERCOT", CatalogVersion: "reward-catalog", MemberPlanID: "essential", ContractVersion: "sim-1",
		PriceText: "SIMULATED", ConsentText: "I accept Essential", ConsentVersion: "sim-1",
		EffectiveAt: begin.Add(-time.Minute), ExpiresAt: begin.Add(time.Hour), CorrelationID: "reward-test"})
	require.NoError(t, err)
	_, err = store.Select(ctx, policy.Selection{ID: "reward-selection", OfferID: "reward-offer", MemberID: "member-1",
		Market: "ERCOT", CatalogVersion: "reward-catalog", MemberPlanID: "essential", PolicyVersion: "reward-policy",
		ConsentText: "I accept Essential", ConsentVersion: "sim-1", ExplanationShown: "Essential backup reserve",
		EffectiveAt: begin.Add(-time.Minute), CorrelationID: "reward-test"})
	require.NoError(t, err)
	harness.track(t)
	harness.advance(t, "ACKNOWLEDGED_OR_UNCERTAIN", "EXECUTING")
	harness.advance(t, "EXECUTING", "VERIFIED")
	harness.advance(t, "VERIFIED", "RECONCILED")
	require.NoError(t, harness.activities.ProduceReport(ctx, harness.input))
	require.NoError(t, harness.activities.ProduceReport(ctx, harness.input))
	var rows int
	var cents int64
	require.NoError(t, harness.pool.QueryRow(ctx, `SELECT count(*),coalesce(sum(amount_cents),0)::bigint
		FROM reward_ledger WHERE event_id = $1`, harness.input.EventID).Scan(&rows, &cents))
	require.Equal(t, 1, rows)
	require.Equal(t, int64(500), cents)
	report, err := storage.LoadPublishedReport(ctx, harness.pool, harness.input.EventID)
	require.NoError(t, err)
	require.NotNil(t, report)
	require.NotNil(t, report.MemberRewardsCents)
	require.Equal(t, int64(500), *report.MemberRewardsCents)
}

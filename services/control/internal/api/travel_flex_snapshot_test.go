package api

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/policy"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTravelFlexFreezeRequiresBoundActiveConsent(t *testing.T) {
	pool := apiTestDatabase(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `INSERT INTO reserve_policies(policy_version, protected_hardware_floor_percent, member_plan_floor_percent, dynamic_override_percent, effective_reserve_percent, effective_at, correlation_id)
		VALUES ('policy-freeze', 10, 65, 0, 65, $1, 'freeze')`, now.Add(-time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO pricing_catalog_snapshots(catalog_version, member_plan_id, market, display_name, reserve_floor_percent, energy_plan, energy_term_months, energy_monthly_charge_cents, battery_plan, battery_term_months, battery_monthly_charge_cents, flexibility_reward_cents, effective_at, correlation_id)
		VALUES ('catalog-freeze', 'plan-freeze', 'TX', 'Freeze', 65, '{}', 0, 100, '{}', 0, 100, 100, $1, 'freeze')`, now.Add(-time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO member_sites(site_id, member_id, bound_at, source, provenance)
		VALUES ('site-flex', 'member-flex', $1, 'SIMULATED', '{"provenance":"SIMULATED"}')`, now.Add(-time.Hour))
	require.NoError(t, err)
	store := policy.New(pool)
	_, err = store.Select(ctx, policy.Selection{ID: "selection-freeze", MemberID: "member-flex", Market: "TX", CatalogVersion: "catalog-freeze", MemberPlanID: "plan-freeze", PolicyVersion: "policy-freeze", ConsentText: "I consent", ConsentVersion: "v1", ExplanationShown: "Backup reserve", EffectiveAt: now.Add(-time.Hour), CorrelationID: "freeze"})
	require.NoError(t, err)
	_, err = store.ScheduleTravelFlex(ctx, policy.TravelFlex{ID: "window-freeze", MemberID: "member-flex", Start: now.Add(-time.Minute), End: now.Add(time.Minute), Timezone: "UTC", TemporaryReservePercent: 20, EarlyReturnAction: policy.RestorePlanReserve, CreditType: "FIXED_EVENT", CreditCents: 100, ConsentText: "I consent to fixed credit", ConsentVersion: "v1", PolicyVersion: "policy-freeze", CorrelationID: "freeze"})
	require.NoError(t, err)
	twin := fleet.NewTwin(time.Minute)
	sites := make([]*gridosv1.AuthorizedSite, 0, 2)
	for _, id := range []string{"site-flex", "site-unbound"} {
		twin.Accept(fleet.SiteState{SiteID: id, ObservedAt: now, OperatingState: fleet.OnGrid, Availability: fleet.Online, EnergyKWh: 8, ReserveKWh: 1})
		sites = append(sites, &gridosv1.AuthorizedSite{Site: &gridosv1.Site{SiteId: id}, Devices: []*gridosv1.Device{{DeviceId: "device-" + id, BatteryParameters: &gridosv1.BatteryParameters{UsableEnergyKwh: 10, MaxChargeKw: 2, MaxDischargeKw: 2, ChargeEfficiency: 1, DischargeEfficiency: 1}}}})
	}
	snapshotter := NewDurableFleetSnapshotter(pool, twin, nil, sites, func() time.Time { return now })
	frozen, err := snapshotter.Freeze(ctx, &gridosv1.DispatchEvent{EventId: "event-freeze"}, &gridosv1.EventRequest{RequestId: "request-freeze", BeginTime: timestamppb.New(now), EndTime: timestamppb.New(now.Add(time.Minute)), TargetKw: 1})
	require.NoError(t, err)
	bound := frozen.Optimization.GetDevices()[0]
	require.True(t, bound.BaseReserveKwh != nil)
	require.True(t, bound.TravelFlexReserveKwh != nil)
	require.Equal(t, 6.5, bound.GetBaseReserveKwh())
	require.Equal(t, 2.0, bound.GetTravelFlexReserveKwh())
	require.Equal(t, 6.5, bound.GetEffectiveReserveKwh())
	require.Equal(t, 6.5, frozen.Canonical.Devices[bound.GetDeviceId()].PlanReserveKWh)
	require.False(t, frozen.Optimization.GetDevices()[1].TravelFlexReserveKwh != nil)
}

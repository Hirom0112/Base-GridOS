package api

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/policy"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/safety"
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
	_, err = store.PresentOffer(ctx, policy.Offer{ID: "offer-freeze", MemberID: "member-flex", Kind: policy.PlanOffer, Market: "TX", CatalogVersion: "catalog-freeze", MemberPlanID: "plan-freeze", ContractVersion: "v1", PriceText: "Fixture catalog terms", ConsentText: "I consent", ConsentVersion: "v1", EffectiveAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), CorrelationID: "freeze"})
	require.NoError(t, err)
	_, err = store.Select(ctx, policy.Selection{ID: "selection-freeze", OfferID: "offer-freeze", MemberID: "member-flex", Market: "TX", CatalogVersion: "catalog-freeze", MemberPlanID: "plan-freeze", PolicyVersion: "policy-freeze", ConsentText: "I consent", ConsentVersion: "v1", ExplanationShown: "Backup reserve", EffectiveAt: now.Add(-time.Hour), CorrelationID: "freeze"})
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
	require.NotNil(t, frozen.Canonical.Devices[bound.GetDeviceId()].TravelFlexReserveKWh)
	require.Equal(t, 2.0, *frozen.Canonical.Devices[bound.GetDeviceId()].TravelFlexReserveKWh)
	require.False(t, frozen.Optimization.GetDevices()[1].TravelFlexReserveKwh != nil)
}

func TestReserveSelectionFromFrozenIsEnforced(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	begin, end := now.Add(time.Minute), now.Add(time.Hour+time.Minute)
	base, flex := 8.0, 6.0
	frozen := &gridosv1.OptimizationRequest{
		RequestedAt: timestamppb.New(now), PlanVersion: 1,
		ReservePolicy: &gridosv1.ReservePolicy{PolicyVersion: "policy-1"},
		Devices: []*gridosv1.DeviceState{{DeviceId: "device-1", UsableEnergyKwh: 10, EnergyKwh: 9, HardwareFloorKwh: 5, EffectiveReserveKwh: 8, BaseReserveKwh: &base, TravelFlexReserveKwh: &flex,
			MaxChargeKw: 2, MaxDischargeKw: 2, ChargeEfficiency: 0.95, DischargeEfficiency: 0.95, AvailabilityProbability: 1, TelemetryObservedAt: timestamppb.New(now)}},
	}
	canonical := CanonicalFromFrozen(frozen)
	plan := &gridosv1.DispatchPlan{DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "device-1", ReserveSelection: gridosv1.ReserveSelection_RESERVE_SELECTION_BASE, SelectedReserveKwh: 8,
		Intervals: []*gridosv1.DeviceScheduleInterval{{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(end), SetpointKw: 1.9, ExpectedEnergyKwh: 7}}}}}
	basePlan, err := safetyPlan(plan, canonical)
	require.NoError(t, err)
	_, violations := safety.Validate(basePlan, canonical)
	require.True(t, violationCodesAPI(violations)[safety.EnergyBelowReserve])
	plan.DeviceSchedules[0].ReserveSelection = gridosv1.ReserveSelection_RESERVE_SELECTION_TRAVEL_FLEX
	plan.DeviceSchedules[0].SelectedReserveKwh = 6
	flexPlan, err := safetyPlan(plan, canonical)
	require.NoError(t, err)
	approval, violations := safety.Validate(flexPlan, canonical)
	require.True(t, approval.Approved, "%+v", violations)
}

func violationCodesAPI(violations []safety.Violation) map[safety.ViolationCode]bool {
	codes := make(map[safety.ViolationCode]bool, len(violations))
	for _, violation := range violations {
		codes[violation.Code] = true
	}
	return codes
}

func TestReserveSelectionPersistsWithPlanVersion(t *testing.T) {
	pool := apiTestDatabase(t)
	ctx := context.Background()
	now := time.Now().UTC()
	begin, end := now.Add(time.Minute), now.Add(time.Hour+time.Minute)
	store := NewPostgresEventStore(pool)
	event, err := store.Create(ctx, &gridosv1.EventRequest{
		RequestId: "reserve-selection-event", EventType: "GRID_SERVICE", BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(end),
		TargetKw: 1, MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT,
		LoadZones: []string{"LZ_AEN"}, CorrelationId: "reserve-selection",
	}, "create-reserve-selection", now)
	require.NoError(t, err)
	request := &gridosv1.OptimizationRequest{EventId: event.GetEventId(), PlanVersion: 1, CorrelationId: "reserve-selection",
		ReservePolicy:       &gridosv1.ReservePolicy{PolicyVersion: "policy-1"},
		EligibilitySnapshot: &gridosv1.EligibilitySnapshot{EligibleDeviceIds: []string{"device-1"}}}
	plan := &gridosv1.DispatchPlan{EventId: event.GetEventId(), PlanVersion: 1, DeviceSchedules: []*gridosv1.DeviceSchedule{{
		DeviceId: "device-1", ReserveSelection: gridosv1.ReserveSelection_RESERVE_SELECTION_TRAVEL_FLEX, SelectedReserveKwh: 6,
		Intervals: []*gridosv1.DeviceScheduleInterval{{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(end), SetpointKw: 1.9, ExpectedEnergyKwh: 7}},
	}}}
	_, err = store.StorePlanned(ctx, event.GetEventId(), request, plan, now)
	require.NoError(t, err)
	_, loaded, err := store.LoadPlan(ctx, event.GetEventId(), 1)
	require.NoError(t, err)
	require.Equal(t, uint64(1), loaded.GetPlanVersion())
	require.Equal(t, gridosv1.ReserveSelection_RESERVE_SELECTION_TRAVEL_FLEX, loaded.GetDeviceSchedules()[0].GetReserveSelection())
	require.Equal(t, 6.0, loaded.GetDeviceSchedules()[0].GetSelectedReserveKwh())
}

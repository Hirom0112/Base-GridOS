package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestRequestedEventHasReportGapBeforePlan(t *testing.T) {
	pool := apiTestDatabase(t)
	now := time.Now().UTC()
	service := NewService(NewPostgresEventStore(pool), fleet.NewTwin(time.Minute), nil, func() time.Time { return now })
	service.SetReportSource(NewPostgresReportSource(pool))
	create := connect.NewRequest(&gridosv1.CreateEventRequestRequest{
		EventRequest: &gridosv1.EventRequest{
			RequestId: "request-before-plan", EventType: "GRID_SERVICE",
			BeginTime: timestamppb.New(now.Add(time.Minute)), EndTime: timestamppb.New(now.Add(time.Hour)),
			TargetKw: 100, MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT,
			LoadZones: []string{"LZ_AEN"},
		},
		IdempotencyKey: "create-before-plan",
	})
	create.Header().Set(roleHeader, "operator")
	created, err := service.CreateEventRequest(context.Background(), create)
	require.NoError(t, err)
	get := connect.NewRequest(&gridosv1.GetEventRequest{EventId: created.Msg.GetEvent().GetEventId()})
	get.Header().Set(roleHeader, "operator")
	response, err := service.GetEvent(context.Background(), get)
	require.NoError(t, err)
	require.Equal(t, gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_REQUESTED, response.Msg.GetEvent().GetState())
	require.Nil(t, response.Msg.GetReport())
	data, err := NewPostgresReportSource(pool).EventReportData(context.Background(), created.Msg.GetEvent().GetEventId())
	require.NoError(t, err)
	require.True(t, hasLiveReportGap(data.DataGaps, "plan_unavailable"))
}

func TestLiveReportUsesFrozenForecastAndStoredDelivery(t *testing.T) {
	pool := apiTestDatabase(t)
	seedAPIEvent(t, pool)
	ctx := context.Background()
	var begin, end time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT begin_time, end_time FROM dispatch_requests WHERE request_id = 'request-restart'`).Scan(&begin, &end))
	coverage := 0.9
	request := &gridosv1.OptimizationRequest{
		EventId: "event-restart", PlanVersion: 3,
		Intervals: []*gridosv1.OptimizationInterval{{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(end)}},
		Sites:     []*gridosv1.ForecastSite{{SiteId: "site-1"}},
		Devices:   []*gridosv1.DeviceState{{DeviceId: "device-1"}},
		Forecast: &gridosv1.ForecastResponse{
			SiteLoads:          []*gridosv1.ForecastSiteLoad{{SiteId: "site-1", IntervalBeginTime: timestamppb.New(begin), LoadKwh: &gridosv1.ForecastValue{Value: 2, Lower: 1, Upper: 3, ModelVersion: "load-baseline-v1", FeatureVersion: "weekday-v1", ValueKind: "modeled_estimate", IntervalCoverage: &coverage}}},
			DeviceAvailability: []*gridosv1.ForecastDeviceAvailability{{DeviceId: "device-1", IntervalBeginTime: timestamppb.New(begin), Probability: &gridosv1.ForecastValue{Value: 0.8, Lower: 0.7, Upper: 0.9, ModelVersion: "availability-v1"}}},
		},
	}
	inputs, err := protojson.Marshal(request)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE input_snapshots SET inputs = $1 WHERE snapshot_id = 'input-restart'`, inputs)
	require.NoError(t, err)
	delivered, err := json.Marshal(report.Delivered{DeliveredMWh: 0.001, DeliveredMW: 0.001, Completeness: 1})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO audit_journal (actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ('reconciliation', 'DELIVERY_VERIFIED', 'event', 'event-restart', $1, 'restart')`, delivered)
	require.NoError(t, err)
	data, err := NewPostgresReportSource(pool).EventReportData(ctx, "event-restart")
	require.NoError(t, err)
	require.NotNil(t, data.Measurement)
	require.Equal(t, 0.002, data.Measurement.BaselineMWh)
	require.Equal(t, 0.002, data.Measurement.BaselineMW)
	require.Equal(t, 0.8, data.Measurement.Availability)
	require.Equal(t, "load-baseline-v1", data.Measurement.BaselineMethod)
	require.NotNil(t, data.Energy)
	require.Equal(t, 0.001, data.Energy.DeliveredMWh)
	require.Equal(t, "load-baseline-v1", data.Versions.Baseline)
	require.Equal(t, "availability-v1", data.Versions.Availability)
	require.Nil(t, data.Economics)
	require.True(t, hasLiveReportGap(data.DataGaps, "modeled_economics_unavailable"))
	require.Equal(t, 0.9, data.Measurement.Confidence)
	require.False(t, hasLiveReportGap(data.DataGaps, "baseline_confidence_unavailable"))
	request.Forecast.SiteLoads[0].LoadKwh.IntervalCoverage = nil
	inputs, err = protojson.Marshal(request)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE input_snapshots SET inputs = $1 WHERE snapshot_id = 'input-restart'`, inputs)
	require.NoError(t, err)
	oldData, err := NewPostgresReportSource(pool).EventReportData(ctx, "event-restart")
	require.NoError(t, err)
	require.True(t, hasLiveReportGap(oldData.DataGaps, "baseline_confidence_unavailable"))
}

func TestLiveReportRewardsAndMarginUseStoredEvidence(t *testing.T) {
	pool := apiTestDatabase(t)
	seedAPIEvent(t, pool)
	ctx := context.Background()
	plan := &gridosv1.DispatchPlan{MarginExplanation: &gridosv1.MarginExplanation{
		ConservativeMargin: -3.25, MarginHurdle: 1,
		Terms: []*gridosv1.MarginTerm{{Name: "DISPATCH_VALUE", Low: -3.25, High: -3.25, Source: "FROZEN_PUBLIC_PRICE"}, {Name: "MEMBER_REWARD", Low: 0, High: 0, Source: "FROZEN_OFFER"}},
	}}
	for _, name := range []string{"AVOIDED_PEAK_COST", "COMMITMENT_RELIABILITY_VALUE", "CHARGING_ENERGY", "INCREMENTAL_DEGRADATION", "PENALTY_EXPOSURE", "SUPPORT_AND_RISK_COST"} {
		plan.MarginExplanation.Terms = append(plan.MarginExplanation.Terms, &gridosv1.MarginTerm{Name: name, Source: "FROZEN_TEST_TERMS"})
	}
	encoded, err := protojson.Marshal(plan)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO plan_versions
		(event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
		VALUES ('event-restart', 4, 'input-restart', 'eligibility-restart', $1, 'solver-report', 'model-report', 'report')`, encoded)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE dispatch_events SET plan_version = 4 WHERE event_id = 'event-restart'`)
	require.NoError(t, err)
	source := NewPostgresReportSource(pool)
	before, err := source.EventReportData(ctx, "event-restart")
	require.NoError(t, err)
	require.Nil(t, before.MemberRewardsCents)
	require.True(t, hasLiveReportGap(before.DataGaps, "reward_unposted"))
	require.NotNil(t, before.Margin)
	require.Equal(t, -3.25, before.Margin.ValueUSD)
	require.False(t, hasLiveReportGap(before.DataGaps, "margin_unavailable"))
	_, err = pool.Exec(ctx, `INSERT INTO pricing_catalog_snapshots
		(catalog_version, member_plan_id, market, energy_plan, energy_term_months, energy_monthly_charge_cents,
		battery_plan, battery_term_months, battery_monthly_charge_cents, flexibility_reward_cents,
		effective_at, correlation_id)
		VALUES ('catalog-report', 'plan-report', 'TX', '{}', 0, 0, '{}', 0, 0, 725, now(), 'report')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO flexibility_offers
		(offer_id, member_id, catalog_version, contract_version, flexibility_reward_cents, price_text,
		effective_at, expires_at, correlation_id, member_plan_id, market, offer_type, consent_text, consent_version,
		energy_monthly_charge_cents, battery_monthly_charge_cents)
		VALUES ('offer-report', 'member-report', 'catalog-report', 'contract-report', 725, 'Fixed event reward',
		now(), now() + interval '1 day', 'report', 'plan-report', 'TX', 'PLAN', 'I agree', 'v1', 0, 0)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO reward_ledger
		(entry_id, member_id, event_id, offer_id, amount_cents, entry_type, recorded_at, correlation_id)
		VALUES ('reward-report', 'member-report', 'event-restart', 'offer-report', 725, 'EARNED', now(), 'report')`)
	require.NoError(t, err)
	after, err := source.EventReportData(ctx, "event-restart")
	require.NoError(t, err)
	require.NotNil(t, after.MemberRewardsCents)
	require.Equal(t, int64(725), *after.MemberRewardsCents)
	require.False(t, hasLiveReportGap(after.DataGaps, "reward_unposted"))
	plan.MarginExplanation.Terms[1].Unavailable = true
	encoded, err = protojson.Marshal(plan)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO plan_versions
		(event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
		VALUES ('event-restart', 5, 'input-restart', 'eligibility-restart', $1, 'solver-report', 'model-report', 'report')`, encoded)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE dispatch_events SET plan_version = 5 WHERE event_id = 'event-restart'`)
	require.NoError(t, err)
	unknown, err := source.EventReportData(ctx, "event-restart")
	require.NoError(t, err)
	require.Nil(t, unknown.Margin)
	require.True(t, hasLiveReportGap(unknown.DataGaps, "margin_unavailable"))
	plan.MarginExplanation.Terms = plan.MarginExplanation.Terms[:1]
	encoded, err = protojson.Marshal(plan)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO plan_versions
		(event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
		VALUES ('event-restart', 6, 'input-restart', 'eligibility-restart', $1, 'solver-report', 'model-report', 'report')`, encoded)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE dispatch_events SET plan_version = 6 WHERE event_id = 'event-restart'`)
	require.NoError(t, err)
	incomplete, err := source.EventReportData(ctx, "event-restart")
	require.NoError(t, err)
	require.Nil(t, incomplete.Margin)
	require.True(t, hasLiveReportGap(incomplete.DataGaps, "margin_unavailable"))
}

func hasLiveReportGap(gaps []report.DataGap, reason string) bool {
	for _, gap := range gaps {
		if gap.Reason == reason && gap.End.After(gap.Begin) {
			return true
		}
	}
	return false
}

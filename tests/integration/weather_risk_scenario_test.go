package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestWeatherStaleAlarmRaiseFloor(t *testing.T) {
	stack := startStack(t, "weather-stale-alarm-raise-floor")
	ctx := context.Background()
	_, err := stack.pool.Exec(ctx, `INSERT INTO risk_policy
		(version, effective_at, expires_at, outage_probability_threshold, telemetry_freshness_seconds,
		gateway_cadence_seconds, weather_floor_percent, outage_floor_percent, stale_floor_percent,
		alarm_floor_percent, communications_floor_percent, health_floor_percent, weather_zone_ugc, provenance)
		VALUES ('risk-policy-weather-scenario', '2020-01-01T00:00:00Z', '2100-01-01T00:00:00Z',
		0.01, 30, 15, 60, 60, 80, 100, 40, 100,
		'{"austin-5000.jsonl":{"SCENT":{"ugc":["TXZ192"],"same":["048453"]}}}', '{"provenance":"SIMULATED"}')`)
	if err != nil {
		t.Fatal(err)
	}
	selected := stack.cohort(t)[0]
	memberID := stack.selectScenarioPlan(t, selected, 10, 20, 100)
	stack.publishTelemetry(t, ctx, stack.cohort(t), time.Now().UTC(), constantStateOfEnergy)
	stack.evaluateRiskNow(t)
	stack.assertRiskFloor(t, memberID, selected.SiteID, "WEATHER", 60)

	stack.processes["gateway"].stop()
	time.Sleep(31 * time.Second)
	stack.evaluateRiskNow(t)
	stack.assertRiskFloor(t, memberID, selected.SiteID, "STALE_TELEMETRY", 80)

	stack.publishOvercurrent(t, selected)
	stack.evaluateRiskNow(t)
	stack.assertRiskFloor(t, memberID, selected.SiteID, "ALARM", 100)
	stack.restart(t, "gateway")

	eventID := fmt.Sprintf("weather-risk-%d", time.Now().UnixNano())
	response := stack.runEvent(t, ctx, eventID, time.Now().UTC())
	stack.assertOutcome(t, ctx, eventID, response)
	report := stack.scenarioReport(t, eventID)
	if report.PlanVersion != 1 || report.ReserveViolationsPrevented != 0 || report.Versions.Policy == "" {
		t.Fatalf("stored weather risk report=%+v", report)
	}
}

func (stack *stack) assertRiskFloor(t *testing.T, memberID, siteID, reason string, floor float64) {
	t.Helper()
	var count int
	if err := stack.pool.QueryRow(context.Background(), `SELECT count(*) FROM reserve_overrides
		WHERE member_id = $1 AND reason = $2 AND reserve_floor_percent = $3`, memberID, reason, floor).Scan(&count); err != nil {
		t.Fatal(err)
	}
	status := stack.memberStatus(t, memberID, siteID)
	if count != 1 || status.GetEffectiveReservePercent() < floor {
		t.Fatalf("%s override count=%d API floor=%g, want %g", reason, count, status.GetEffectiveReservePercent(), floor)
	}
}

func (stack *stack) publishOvercurrent(t *testing.T, device FleetDevice) {
	t.Helper()
	now := time.Now().UTC()
	observation := &gridosv1.TelemetryObservation{ObservationId: fmt.Sprintf("overcurrent-%s-%d", device.DeviceID, now.UnixNano()),
		DeviceId: device.DeviceID, Sequence: uint64(now.UnixNano()), ObservationTime: timestamppb.New(now),
		ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT, StateOfEnergyPercent: 74,
		OperatingState: &gridosv1.TelemetryObservation_OffGridOvercurrent{OffGridOvercurrent: &gridosv1.OffGridOvercurrent{
			ObservedAt: timestamppb.New(now), EstimatedBackupHoursAtCurrentUsage: 4,
			EstimatedBackupHoursAt_750Watts: 12, OvercurrentLimitKw: 5}}}
	request := connect.NewRequest(&gridosv1.PublishTelemetryRequest{GatewayId: gatewayID, Observations: []*gridosv1.TelemetryObservation{observation}})
	request.Header().Set("Authorization", gatewayToken)
	response, err := gridosv1connect.NewTelemetryServiceClient(stack.client, stack.controlURL).PublishTelemetry(context.Background(), request)
	if err != nil || response.Msg.GetDurableReceiptId() == "" {
		t.Fatalf("overcurrent receipt=%+v error=%v", response, err)
	}
}

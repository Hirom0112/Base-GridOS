package integration

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAnomalySignalLabel(t *testing.T) {
	stack := startStack(t, "anomaly-signal-label")
	ctx := context.Background()
	stack.seedScenarioRiskPolicy(t)
	cohort := stack.cohort(t)
	selected := cohort[0]
	memberID := "member-" + selected.SiteID[len("site_"):]
	now := time.Now().UTC()
	client := gridosv1connect.NewMemberServiceClient(stack.client, stack.controlURL)
	preference := connect.NewRequest(&gridosv1.SetAnomalyPreferenceRequest{MemberId: memberID,
		IdempotencyKey: "anomaly-preference-" + memberID, OptedIn: true, ConsentText: "I opt in to anomaly signals",
		ConsentVersion: "anomaly-consent-v1", BaselineUpperKw: 1,
		BaselineBegin: timestamppb.New(now.Add(-time.Minute)), BaselineEnd: timestamppb.New(now.Add(6 * time.Minute)),
		EffectiveAt: timestamppb.New(now.Add(-time.Minute)), ExpiresAt: timestamppb.New(now.Add(6 * time.Minute)),
		CorrelationId: memberID})
	memberHeaders(preference.Header(), memberID)
	if _, err := client.SetAnomalyPreference(ctx, preference); err != nil {
		t.Fatal(err)
	}
	away := connect.NewRequest(&gridosv1.ScheduleAwayRequest{MemberId: memberID,
		IdempotencyKey: "anomaly-away-" + memberID, StartTime: timestamppb.New(now.Add(-time.Minute)),
		EndTime: timestamppb.New(now.Add(6 * time.Minute)), ConsentVersion: preference.Msg.ConsentVersion, CorrelationId: memberID})
	memberHeaders(away.Header(), memberID)
	if _, err := client.ScheduleAway(ctx, away); err != nil {
		t.Fatal(err)
	}
	stack.publishTelemetry(t, ctx, cohort, time.Now().UTC(), constantStateOfEnergy)
	stack.publishHomeLoad(t, selected, 0.5)
	stack.evaluateRiskNow(t)
	if alerts := stack.memberAlerts(t, memberID); len(alerts) != 0 {
		t.Fatalf("normal measured load produced alerts=%+v", alerts)
	}
	stack.publishHomeLoad(t, selected, 2)
	stack.evaluateRiskNow(t)
	alerts := stack.memberAlerts(t, memberID)
	if len(alerts) != 1 || alerts[0].GetDescription() != "energy anomaly signal" || alerts[0].GetMemberId() != memberID || alerts[0].GetConsentVersion() != "anomaly-consent-v1" {
		var latest string
		var load float64
		_ = stack.pool.QueryRow(ctx, `SELECT observation_id, COALESCE((payload->'powerFlow'->>'toHomeKw')::double precision, 0)
			FROM telemetry_observations WHERE device_id = $1 ORDER BY observed_at DESC, sequence DESC LIMIT 1`, selected.DeviceID).Scan(&latest, &load)
		var preference, away int
		_ = stack.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM member_anomaly_preferences WHERE member_id = $1),
			(SELECT count(*) FROM member_away_periods WHERE member_id = $1)`, memberID).Scan(&preference, &away)
		t.Fatalf("anomaly API alerts=%+v latest=%s load=%g preference=%d away=%d", alerts, latest, load, preference, away)
	}
	eventID := fmt.Sprintf("anomaly-%d", time.Now().UnixNano())
	response := stack.runEvent(t, ctx, eventID, time.Now().UTC())
	stack.assertOutcome(t, ctx, eventID, response)
	report := stack.scenarioReport(t, eventID)
	if report.PlanVersion != 1 || report.ReserveViolationsPrevented != 0 {
		t.Fatalf("stored anomaly scenario report=%+v", report)
	}
}

func (stack *stack) seedScenarioRiskPolicy(t *testing.T) {
	t.Helper()
	_, err := stack.pool.Exec(context.Background(), `INSERT INTO risk_policy
		(version, effective_at, expires_at, outage_probability_threshold, telemetry_freshness_seconds,
		gateway_cadence_seconds, weather_floor_percent, outage_floor_percent, stale_floor_percent,
		alarm_floor_percent, communications_floor_percent, health_floor_percent, weather_zone_ugc, provenance)
		VALUES ('risk-policy-scenario-1', '2020-01-01T00:00:00Z', '2100-01-01T00:00:00Z',
		0.01, 30, 15, 60, 60, 40, 100, 40, 100,
		'{"austin-5000.jsonl":{"SCENT":{"ugc":["TXZ192"],"same":["048453"]}}}', '{"provenance":"SIMULATED"}')`)
	if err != nil {
		t.Fatal(err)
	}
}

func (stack *stack) publishHomeLoad(t *testing.T, device FleetDevice, load float64) {
	t.Helper()
	now := time.Now().UTC()
	observation := &gridosv1.TelemetryObservation{ObservationId: fmt.Sprintf("home-load-%s-%d", device.DeviceID, now.UnixNano()),
		DeviceId: device.DeviceID, Sequence: uint64(now.UnixNano()), ObservationTime: timestamppb.New(now),
		ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT, StateOfEnergyPercent: 74,
		PowerFlow:      &gridosv1.PowerFlow{FromGridKw: load, NonSolarToHomeKw: load, ToHomeKw: load},
		OperatingState: &gridosv1.TelemetryObservation_OnGrid{OnGrid: &gridosv1.OnGrid{ObservedAt: timestamppb.New(now), EstimatedBackupHoursAtCurrentUsage: 4, EstimatedBackupHoursAt_750Watts: 12}}}
	request := connect.NewRequest(&gridosv1.PublishTelemetryRequest{GatewayId: gatewayID, Observations: []*gridosv1.TelemetryObservation{observation}})
	request.Header().Set("Authorization", gatewayToken)
	response, err := gridosv1connect.NewTelemetryServiceClient(stack.client, stack.controlURL).PublishTelemetry(context.Background(), request)
	if err != nil || response.Msg.GetDurableReceiptId() == "" {
		t.Fatalf("home-load telemetry receipt=%+v error=%v", response, err)
	}
}

func (stack *stack) evaluateRiskNow(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "temporal", "workflow", "execute", "--task-queue", stack.databaseName,
		"--type", "RiskOverrides", "--workflow-id", fmt.Sprintf("%s-risk-%d", stack.databaseName, time.Now().UnixNano()))
	command.Dir = stack.root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("risk workflow: %v: %s", err, output)
	}
}

func (stack *stack) memberAlerts(t *testing.T, memberID string) []*gridosv1.HomeActivityAlert {
	t.Helper()
	request := connect.NewRequest(&gridosv1.ListHomeActivityAlertsRequest{MemberId: memberID})
	memberHeaders(request.Header(), memberID)
	response, err := gridosv1connect.NewMemberServiceClient(stack.client, stack.controlURL).ListHomeActivityAlerts(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	return response.Msg.GetAlerts()
}

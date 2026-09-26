package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
)

func TestUnderReservedExcluded(t *testing.T) {
	stack := startStack(t, "under-reserved-excluded")
	ctx := context.Background()
	now := time.Now().UTC()
	cohort := stack.cohort(t)
	underReserved := make(map[string]bool)
	for _, device := range cohort {
		if len(underReserved) < 60 && device.ReservePreferencePercent >= 10 {
			underReserved[device.DeviceID] = true
		}
	}
	stateOfEnergy := func(device FleetDevice) float64 {
		if underReserved[device.DeviceID] {
			return device.ReservePreferencePercent - 5
		}
		return 74
	}
	stack.publishTelemetry(t, ctx, cohort, now, stateOfEnergy)
	expected := make(map[string]bool)
	for _, device := range cohort {
		if stateOfEnergy(device) <= device.ReservePreferencePercent {
			expected[device.DeviceID] = true
		}
	}
	eventID := fmt.Sprintf("under-reserved-%d", now.UnixNano())
	response := stack.runEvent(t, ctx, eventID, now)
	excluded := make(map[string]bool)
	for deviceID, reason := range stack.planExclusions(t, ctx, eventID) {
		if reason == "EXCLUSION_REASON_RESERVE" {
			excluded[deviceID] = true
		}
	}
	if len(excluded) != len(expected) {
		t.Fatalf("reserve exclusions = %d, expected %d", len(excluded), len(expected))
	}
	for deviceID := range expected {
		if !excluded[deviceID] {
			t.Fatalf("device %s below reserve was not excluded", deviceID)
		}
	}
	if reported := exclusionCount(response, gridosv1.ExclusionReason_EXCLUSION_REASON_RESERVE); reported != uint64(len(expected)) {
		t.Fatalf("reported reserve exclusions = %d, expected %d", reported, len(expected))
	}
	for commandID, command := range stack.commandStates(t, ctx, eventID) {
		if expected[command.deviceID] {
			t.Fatalf("command %s targets a device below reserve", commandID)
		}
	}
	stack.assertOutcome(t, ctx, eventID, response)
}

func TestWorkerTermination(t *testing.T) {
	stack := startStack(t, "worker-termination")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.cohort(t), now, constantStateOfEnergy)
	eventID := fmt.Sprintf("worker-termination-%d", now.UnixNano())
	stack.createEvent(t, ctx, eventID)
	stack.waitEventState(t, ctx, eventID, "VALIDATED")
	stack.approveEvent(t, ctx, eventID, now)
	stack.launchEvent(t, ctx, eventID, now)
	interrupted := stack.waitEventState(t, ctx, eventID, "SENT").GetEvent().GetState()
	stack.processes["worker"].kill()
	if interrupted == gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_REPORTED {
		t.Fatal("worker finished the event before it was terminated")
	}
	stack.restart(t, "worker")
	response := stack.waitEventState(t, ctx, eventID, "REPORTED")
	commands := stack.commandStates(t, ctx, eventID)
	launched := 0
	for _, command := range commands {
		if command.generation == 1 {
			launched++
		}
	}
	if launched == 0 || len(commands) != 2*launched {
		t.Fatalf("commands after resume = %d, launched = %d", len(commands), launched)
	}
	if retained := stack.gatewayCommands(t, ctx, eventID); retained != len(commands) {
		t.Fatalf("gateway holds %d physical intents for %d commands", retained, len(commands))
	}
	stack.assertOutcome(t, ctx, eventID, response)
}

func TestGatewayRestart(t *testing.T) {
	stack := startStack(t, "gateway-restart")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.cohort(t), now, constantStateOfEnergy)
	eventID := fmt.Sprintf("gateway-restart-%d", now.UnixNano())
	stack.createEvent(t, ctx, eventID)
	stack.waitEventState(t, ctx, eventID, "VALIDATED")
	stack.approveEvent(t, ctx, eventID, now)
	stack.launchEvent(t, ctx, eventID, now)
	interrupted := stack.waitEventState(t, ctx, eventID, "SENT").GetEvent().GetState()
	stack.processes["gateway"].kill()
	if interrupted == gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_REPORTED {
		t.Fatal("event completed before the gateway was restarted")
	}
	acknowledged := stack.commandStates(t, ctx, eventID)
	stack.restart(t, "gateway")
	if retained := stack.gatewayCommands(t, ctx, eventID); retained < len(acknowledged) {
		t.Fatalf("gateway retained %d of %d commands across the restart", retained, len(acknowledged))
	}
	response := stack.waitEventState(t, ctx, eventID, "REPORTED")
	commands := stack.commandStates(t, ctx, eventID)
	if retained := stack.gatewayCommands(t, ctx, eventID); retained != len(commands) || len(commands) <= len(acknowledged) {
		t.Fatalf("gateway holds %d of %d commands after the restart, %d before", retained, len(commands), len(acknowledged))
	}
	stack.assertOutcome(t, ctx, eventID, response)
}

func TestHoustonTwentyPercentOffline(t *testing.T) {
	stack := startStack(t, "houston-20pct-offline")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.cohort(t), now, constantStateOfEnergy)
	eventID := fmt.Sprintf("houston-offline-%d", now.UnixNano())
	response := stack.runEvent(t, ctx, eventID, now)
	var missing, present int
	err := stack.pool.QueryRow(ctx, `SELECT
		count(*) FILTER (WHERE observation.new_values->>'valueState' = 'VALUE_STATE_MISSING'),
		count(*) FILTER (WHERE observation.new_values->>'valueState' = 'VALUE_STATE_PRESENT')
		FROM audit_journal AS observation
		WHERE observation.action = 'TELEMETRY_RECEIVED'
		AND observation.actor_id IN (SELECT device_id FROM command_intents WHERE event_id = $1)
		AND observation.occurred_at BETWEEN $2 AND $3`, eventID, stack.scenario.Event.StartAt, stack.scenario.Event.EndAt).Scan(&missing, &present)
	if err != nil {
		t.Fatal(err)
	}
	if missing == 0 || present == 0 {
		t.Fatalf("scheduled Houston telemetry: %d missing, %d present", missing, present)
	}
	var affectedDevice string
	var missingAt time.Time
	err = stack.pool.QueryRow(ctx, `SELECT observation.actor_id, observation.occurred_at
		FROM audit_journal AS observation
		WHERE observation.action = 'TELEMETRY_RECEIVED'
		AND observation.new_values->>'valueState' = 'VALUE_STATE_MISSING'
		AND observation.actor_id IN (SELECT device_id FROM command_intents WHERE event_id = $1)
		AND observation.occurred_at BETWEEN $2 AND $3 LIMIT 1`, eventID, stack.scenario.Event.StartAt, stack.scenario.Event.EndAt).Scan(&affectedDevice, &missingAt)
	if err != nil {
		t.Fatal(err)
	}
	var adjacent int
	err = stack.pool.QueryRow(ctx, `SELECT count(*) FROM audit_journal
		WHERE action = 'TELEMETRY_RECEIVED' AND actor_id = $1
		AND new_values->>'valueState' = 'VALUE_STATE_PRESENT'
		AND occurred_at BETWEEN $2 AND $3`, affectedDevice, missingAt.Add(-10*time.Second), missingAt.Add(10*time.Second)).Scan(&adjacent)
	if err != nil {
		t.Fatal(err)
	}
	if adjacent == 0 {
		t.Fatalf("missing Houston observation for %s has no adjacent live tick", affectedDevice)
	}
	stack.assertOutcome(t, ctx, eventID, response)
}

func TestLostAckStillExecuting(t *testing.T) {
	stack := startStack(t, "lost-ack-still-executing")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.cohort(t), now, constantStateOfEnergy)
	eventID := fmt.Sprintf("lost-ack-%d", now.UnixNano())
	stack.createEvent(t, ctx, eventID)
	stack.waitEventState(t, ctx, eventID, "VALIDATED")
	stack.approveEvent(t, ctx, eventID, now)
	injectionOffset := stack.scenario.Injections[0].At.Sub(stack.scenario.Clock.StartAt)
	time.Sleep(time.Until(stack.processes["gateway"].startedAt.Add(injectionOffset + 2*time.Second)))
	stack.launchEvent(t, ctx, eventID, time.Now().UTC())
	response := stack.waitEventState(t, ctx, eventID, "REPORTED")
	var uncertain, acknowledged int
	err := stack.pool.QueryRow(ctx, `SELECT
		count(*) FILTER (WHERE state.state = 'UNCERTAIN'),
		count(*) FILTER (WHERE state.state = 'ACKNOWLEDGED')
		FROM command_states AS state JOIN command_intents AS intent USING (command_id)
		WHERE intent.event_id = $1`, eventID).Scan(&uncertain, &acknowledged)
	if err != nil {
		t.Fatal(err)
	}
	if uncertain == 0 || acknowledged == 0 {
		t.Fatalf("command history has %d uncertain and %d acknowledged states", uncertain, acknowledged)
	}
	activeStart := stack.processes["gateway"].startedAt.Add(injectionOffset)
	var sentDuringFault int
	err = stack.pool.QueryRow(ctx, `SELECT count(*) FROM command_states AS state
		JOIN command_intents AS intent USING (command_id)
		WHERE intent.event_id = $1 AND state.state = 'SENT'
		AND state.recorded_at BETWEEN $2 AND $3`, eventID, activeStart, activeStart.Add(5*time.Second)).Scan(&sentDuringFault)
	if err != nil {
		t.Fatal(err)
	}
	if sentDuringFault == 0 {
		t.Fatal("no command was sent during the injected gateway tick")
	}
	stack.assertOutcome(t, ctx, eventID, response)
}

func TestOldExpiryNewerPending(t *testing.T) {
	stack := startStack(t, "old-command-expiry-newer-pending")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.cohort(t), now, constantStateOfEnergy)
	eventID := fmt.Sprintf("old-expiry-%d", now.UnixNano())
	stack.createEvent(t, ctx, eventID)
	stack.waitEventState(t, ctx, eventID, "VALIDATED")
	stack.approveEvent(t, ctx, eventID, now)
	injectionOffset := stack.scenario.Injections[0].At.Sub(stack.scenario.Clock.StartAt)
	time.Sleep(time.Until(stack.processes["gateway"].startedAt.Add(injectionOffset + 2*time.Second)))
	stack.launchEvent(t, ctx, eventID, time.Now().UTC())
	response := stack.waitEventState(t, ctx, eventID, "REPORTED")
	var paired, uncertain int
	err := stack.pool.QueryRow(ctx, `SELECT
		count(DISTINCT old.device_id),
		count(DISTINCT newer.command_id) FILTER (WHERE newer_state.state = 'UNCERTAIN')
		FROM command_intents AS old
		JOIN command_intents AS newer ON newer.event_id = old.event_id AND newer.device_id = old.device_id AND newer.generation = 2
		JOIN command_states AS newer_state ON newer_state.command_id = newer.command_id
		WHERE old.event_id = $1 AND old.generation = 1 AND old.expires_at <= newer.effective_at`, eventID).Scan(&paired, &uncertain)
	if err != nil {
		t.Fatal(err)
	}
	if paired == 0 || uncertain == 0 {
		t.Fatalf("expired old commands paired with newer commands = %d, uncertain newer commands = %d", paired, uncertain)
	}
	commands := stack.commandStates(t, ctx, eventID)
	if retained := stack.gatewayCommands(t, ctx, eventID); retained != len(commands) {
		t.Fatalf("gateway retained %d of %d command intents", retained, len(commands))
	}
	stack.assertOutcome(t, ctx, eventID, response)
}

func TestOutageReplay(t *testing.T) {
	t.Skip("pending 2D.4: the harness does not yet stream simulator telemetry through control to drive the buffered replay")
}

func TestMeasurementGapUnknown(t *testing.T) {
	t.Skip("pending director: VERIFIED 2B.7: the report carries no uncertain intervals until the reconciliation wiring lands, so a measurement gap has no observable outcome")
}

func (stack *stack) assertOutcome(t *testing.T, ctx context.Context, eventID string, response *gridosv1.GetEventResponse) {
	t.Helper()
	expected := stack.scenario.Expected
	if state := response.GetEvent().GetState().String(); state != "DISPATCH_EVENT_STATE_"+expected.FinalEventState {
		t.Fatalf("event %s ended %s, scenario expects %s", eventID, state, expected.FinalEventState)
	}
	if violations := len(response.GetSafetyViolations()); violations != expected.ReserveViolations {
		t.Fatalf("event %s has %d safety violations, scenario allows %d", eventID, violations, expected.ReserveViolations)
	}
	report := response.GetReport()
	if !expected.AllowsShortfall && report.GetAcknowledgedMw() < report.GetRequestedMw() {
		t.Fatalf("event %s acknowledged %.3f MW of %.3f requested, scenario allows no shortfall", eventID, report.GetAcknowledgedMw(), report.GetRequestedMw())
	}
	if checked := stack.assertAuditChain(t, ctx, eventID); checked == 0 {
		t.Fatalf("event %s has no command chains", eventID)
	}
}

func exclusionCount(response *gridosv1.GetEventResponse, reason gridosv1.ExclusionReason) uint64 {
	for _, group := range response.GetExclusions() {
		if group.GetReason() == reason {
			return group.GetCount()
		}
	}
	return 0
}

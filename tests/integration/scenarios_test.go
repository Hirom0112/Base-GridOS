package integration

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/encoding/protojson"
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
	stack := startStack(t, "network-outage-sqlite-replay")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.cohort(t), now, constantStateOfEnergy)
	eventID := fmt.Sprintf("outage-replay-%d", now.UnixNano())
	stack.createEvent(t, ctx, eventID)
	stack.waitEventState(t, ctx, eventID, "VALIDATED")
	stack.approveEvent(t, ctx, eventID, now)
	stack.launchEvent(t, ctx, eventID, now)
	database, err := sql.Open("sqlite", stack.gatewayDB)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	var bufferedID string
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		var count int
		if err = database.QueryRowContext(ctx, "SELECT count(*) FROM telemetry_buffer").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			bufferedID = ""
			time.Sleep(statePoll)
			continue
		}
		var payload []byte
		if err = database.QueryRowContext(ctx, "SELECT observation_id, payload FROM telemetry_buffer").Scan(&bufferedID, &payload); err != nil {
			t.Fatal(err)
		}
		observation := new(gridosv1.TelemetryObservation)
		if err = protojson.Unmarshal(payload, observation); err != nil {
			t.Fatal(err)
		}
		if observation.GetSourceTime().AsTime().Equal(stack.scenario.Injections[0].At) {
			time.Sleep(500 * time.Millisecond)
			if err = database.QueryRowContext(ctx, "SELECT count(*) FROM telemetry_buffer WHERE observation_id = ?", bufferedID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count == 1 {
				break
			}
		}
		bufferedID = ""
		time.Sleep(statePoll)
	}
	if bufferedID == "" {
		t.Fatal("no delayed observation remained in the gateway SQLite buffer")
	}
	response := stack.waitEventState(t, ctx, eventID, "REPORTED")
	var retained, replayed, live int
	if err = database.QueryRowContext(ctx, "SELECT count(*) FROM telemetry_buffer WHERE observation_id = ?", bufferedID).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if err = stack.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE new_values->>'observationId' = $1),
		count(*) FILTER (WHERE new_values->>'observationId' <> $1)
		FROM audit_journal WHERE action = 'TELEMETRY_RECEIVED' AND occurred_at = $2`, bufferedID, stack.scenario.Injections[0].At).Scan(&replayed, &live); err != nil {
		t.Fatal(err)
	}
	if retained != 0 || replayed != 1 || live == 0 {
		t.Fatalf("buffered observation %s: retained %d, replayed %d, live controls %d", bufferedID, retained, replayed, live)
	}
	stack.assertOutcome(t, ctx, eventID, response)
}

func TestMeasurementGapUnknown(t *testing.T) {
	stack := startStack(t, "measurement-gap-unknown")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.cohort(t), now, constantStateOfEnergy)
	eventID := fmt.Sprintf("measurement-gap-%d", now.UnixNano())
	response := stack.runEvent(t, ctx, eventID, now)
	var affectedDevice string
	for _, injection := range stack.scenario.Injections {
		var live int
		if err := stack.pool.QueryRow(ctx, `SELECT count(*) FROM audit_journal
			WHERE action = 'TELEMETRY_RECEIVED' AND occurred_at = $1
			AND actor_id IN (SELECT device_id FROM command_intents WHERE event_id = $2)`, injection.At, eventID).Scan(&live); err != nil {
			t.Fatal(err)
		}
		if live == 0 {
			continue
		}
		err := stack.pool.QueryRow(ctx, `SELECT COALESCE((SELECT intent.device_id FROM command_intents AS intent
			WHERE intent.event_id = $1 AND intent.generation = 1
			AND EXISTS (SELECT 1 FROM audit_journal AS earlier WHERE earlier.action = 'TELEMETRY_RECEIVED'
				AND earlier.actor_id = intent.device_id AND earlier.occurred_at BETWEEN $2 AND $3)
			AND NOT EXISTS (SELECT 1 FROM audit_journal AS missing WHERE missing.action = 'TELEMETRY_RECEIVED'
				AND missing.actor_id = intent.device_id AND missing.occurred_at = $3)
			LIMIT 1), '')`, eventID, stack.scenario.Event.StartAt, injection.At).Scan(&affectedDevice)
		if err != nil {
			t.Fatal(err)
		}
		if affectedDevice != "" {
			break
		}
	}
	if affectedDevice == "" {
		t.Fatal("no scheduled device lost an observation during a live injected tick")
	}
	uncertain := false
	for _, interval := range response.GetReport().GetUncertainIntervals() {
		if interval.GetDeviceId() == affectedDevice && interval.GetEndTime().AsTime().Sub(interval.GetBeginTime().AsTime()) > 30*time.Second {
			uncertain = true
			break
		}
	}
	if !uncertain {
		t.Fatalf("dropped telemetry for %s did not produce an unknown interval over 30 seconds", affectedDevice)
	}
	stack.assertOutcome(t, ctx, eventID, response)
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

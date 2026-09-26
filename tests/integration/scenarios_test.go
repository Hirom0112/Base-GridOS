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
	t.Skip("pending director: VERIFIED 2B.7: the report carries no delivered or lost MW until the reconciliation wiring lands, so an outage over 20 percent of Houston schedules has no observable outcome")
}

func TestLostAckStillExecuting(t *testing.T) {
	t.Skip("pending director: VERIFIED 2B.7: the publisher continue-on-uncertain fix and the UNCERTAIN to ACKNOWLEDGED transition are part of 2B.7")
}

func TestOldExpiryNewerPending(t *testing.T) {
	t.Skip("pending director: VERIFIED 2B.7: needs the lost-acknowledgement path above to hold a newer command UNCERTAIN while the older one expires")
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

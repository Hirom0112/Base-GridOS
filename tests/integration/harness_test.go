package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
)

const (
	publisherBatch = 100
	cohortSize     = 400
)

func TestMain(m *testing.M) {
	code := m.Run()
	removeBinaries()
	os.Exit(code)
}

func TestHarness(t *testing.T) {
	ctx := context.Background()
	var databaseName string
	t.Cleanup(func() { assertDatabaseDropped(t, ctx, databaseName) })
	stack := startStack(t, "gateway-restart")
	databaseName = stack.databaseName
	now := time.Now().UTC()
	cohort := stack.cohort(t)
	stack.publishTelemetry(t, ctx, cohort, now, constantStateOfEnergy)
	stack.assertDispatchable(t, ctx)
	eventID := fmt.Sprintf("harness-%d", now.UnixNano())
	created := stack.createEvent(t, ctx, eventID)
	if created.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_REQUESTED {
		t.Fatalf("created event state = %s", created.GetState())
	}
	validated := stack.waitEventState(t, ctx, eventID, "VALIDATED").GetEvent()
	if validated.GetPlanVersion() != 1 {
		t.Fatalf("validated event plan = %d", validated.GetPlanVersion())
	}
	stack.approveEvent(t, ctx, eventID, now)
	launched := stack.launchEvent(t, ctx, eventID, now)
	if launched.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_APPROVED {
		t.Fatalf("launched event state = %s", launched.GetState())
	}
	report := stack.waitEventState(t, ctx, eventID, "REPORTED").GetReport()
	if report.GetRequestedMw() != stack.scenario.Event.TargetMW || report.GetAcknowledgedMw() <= 0 {
		t.Fatalf("report power = %#v", report)
	}
	commands := stack.commandStates(t, ctx, eventID)
	if len(commands) == 0 {
		t.Fatal("launch persisted no commands")
	}
	launchedCommands := 0
	for commandID, command := range commands {
		if command.state != "ACKNOWLEDGED" {
			t.Fatalf("command %s ended %s", commandID, command.state)
		}
		if command.generation == 1 {
			launchedCommands++
		}
	}
	if launchedCommands == 0 || launchedCommands > publisherBatch {
		t.Fatalf("launch persisted %d commands, publisher batch is %d", launchedCommands, publisherBatch)
	}
	if retained := stack.gatewayCommands(t, ctx, eventID); retained != len(commands) {
		t.Fatalf("gateway retained %d of %d commands", retained, len(commands))
	}
}

func constantStateOfEnergy(FleetDevice) float64 {
	return 74
}

func (stack *stack) runEvent(t *testing.T, ctx context.Context, eventID string, now time.Time) *gridosv1.GetEventResponse {
	t.Helper()
	stack.createEvent(t, ctx, eventID)
	stack.waitEventState(t, ctx, eventID, "VALIDATED")
	stack.approveEvent(t, ctx, eventID, now)
	stack.launchEvent(t, ctx, eventID, now)
	return stack.waitEventState(t, ctx, eventID, stack.scenario.Expected.FinalEventState)
}

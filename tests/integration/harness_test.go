package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
)

func TestMain(m *testing.M) {
	code := m.Run()
	removeBinaries()
	os.Exit(code)
}

func TestHarness(t *testing.T) {
	stack := startStack(t, "gateway-restart")
	ctx := context.Background()
	t.Cleanup(func() { assertDatabaseDropped(t, ctx, stack.databaseName) })
	now := time.Now().UTC()
	cohort := stack.cohort(t, 120)
	stack.publishTelemetry(t, ctx, cohort, now, 74)
	stack.assertDispatchable(t, ctx)
	eventID := fmt.Sprintf("harness-%d", now.UnixNano())
	created := stack.createEvent(t, ctx, eventID, now)
	if created.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_VALIDATED || created.GetPlanVersion() != 1 {
		t.Fatalf("created event state = %s plan %d", created.GetState(), created.GetPlanVersion())
	}
	stack.approveEvent(t, ctx, eventID, now)
	launched := stack.launchEvent(t, ctx, eventID, now)
	if launched.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_ACKNOWLEDGED_OR_UNCERTAIN {
		t.Fatalf("launched event state = %s", launched.GetState())
	}
	report := stack.getEvent(t, ctx, eventID).GetReport()
	if report.GetRequestedMw() != stack.scenario.Event.TargetMW || report.GetAcknowledgedMw() <= 0 {
		t.Fatalf("report power = %#v", report)
	}
	commands := stack.commandStates(t, ctx, eventID)
	if len(commands) == 0 {
		t.Fatal("launch persisted no commands")
	}
	for id, state := range commands {
		if state != "ACKNOWLEDGED" {
			t.Fatalf("command %s state = %s", id, state)
		}
	}
	if retained := stack.gatewayCommands(t, ctx, eventID); retained != len(commands) {
		t.Fatalf("gateway retained %d of %d commands", retained, len(commands))
	}
}

func TestHoustonTwentyPercentOffline(t *testing.T) {
	t.Skip("pending director verification of 2A.2, 2B.3, and 2C.2")
}

func TestLostAckStillExecuting(t *testing.T) {
	t.Skip("pending director verification of 2A.2, 2B.3, and 2C.2")
}

func TestOldExpiryNewerPending(t *testing.T) {
	t.Skip("pending director verification of 2A.2, 2B.3, and 2C.2")
}

func TestWorkerTermination(t *testing.T) {
	t.Skip("pending director verification of 2A.4 and 2B.6")
}

func TestGatewayRestart(t *testing.T) {
	t.Skip("pending director verification of 2A.4 and 2B.6")
}

func TestOutageReplay(t *testing.T) {
	t.Skip("pending director verification of 2A.4 and 2B.6")
}

func TestMeasurementGapUnknown(t *testing.T) {
	t.Skip("pending director verification of 2C.4")
}

func TestUnderReservedExcluded(t *testing.T) {
	t.Skip("pending director verification of 2C.4")
}

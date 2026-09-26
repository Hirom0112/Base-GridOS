package integration

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestAuditChain(t *testing.T) {
	unbroken := []transition{{"REQUESTED", "PLANNED"}, {"PLANNED", "VALIDATED"}, {"VALIDATED", "APPROVED"}}
	if err := chainBreak(unbroken, "REQUESTED", "APPROVED"); err != nil {
		t.Fatalf("unbroken chain reported %v", err)
	}
	gap := []transition{{"REQUESTED", "PLANNED"}, {"VALIDATED", "APPROVED"}}
	if err := chainBreak(gap, "REQUESTED", "APPROVED"); err == nil {
		t.Fatal("skipped transition went unreported")
	}
	if err := chainBreak(unbroken, "REQUESTED", "SENT"); err == nil {
		t.Fatal("chain ending short of the stored state went unreported")
	}
	illegal := []string{"PERSISTED", "ACKNOWLEDGED"}
	if err := commandChainBreak(illegal); err == nil {
		t.Fatal("command chain skipping SENT went unreported")
	}
	if err := commandChainBreak([]string{"PERSISTED", "SENT", "ACKNOWLEDGED"}); err != nil {
		t.Fatalf("legal command chain reported %v", err)
	}

	stack := startStack(t, "lost-ack-still-executing")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.cohort(t, 400), now, 74)
	eventID := fmt.Sprintf("audit-%d", now.UnixNano())
	stack.createEvent(t, ctx, eventID, now)
	stack.approveEvent(t, ctx, eventID, now)
	stack.launchEvent(t, ctx, eventID, now)
	stack.assertAuditChain(t, ctx, eventID)
}

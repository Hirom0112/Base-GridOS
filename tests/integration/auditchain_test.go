package integration

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestAuditChain(t *testing.T) {
	unbroken := []transition{{"REQUESTED", "PLANNED"}, {"PLANNED", "VALIDATED"}, {"VALIDATED", "APPROVED"}}
	if err := chainBreak(unbroken, "APPROVED"); err != nil {
		t.Fatalf("unbroken chain reported %v", err)
	}
	gap := []transition{{"REQUESTED", "PLANNED"}, {"VALIDATED", "APPROVED"}}
	if err := chainBreak(gap, "APPROVED"); err == nil {
		t.Fatal("skipped transition went unreported")
	}
	if err := chainBreak(unbroken, "SENT"); err == nil {
		t.Fatal("chain ending short of the stored state went unreported")
	}
	illegal := []string{"PERSISTED", "ACKNOWLEDGED"}
	if err := commandChainBreak(illegal); err == nil {
		t.Fatal("command chain skipping SENT went unreported")
	}
	if err := commandChainBreak([]string{"PERSISTED", "SENT", "ACKNOWLEDGED"}); err != nil {
		t.Fatalf("legal command chain reported %v", err)
	}
	if err := commandChainBreak([]string{"PERSISTED", "SENT", "UNCERTAIN", "ACKNOWLEDGED"}); err != nil {
		t.Fatalf("late acceptance after uncertainty reported %v", err)
	}

	stack := startStack(t, "lost-ack-still-executing")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.cohort(t), now, constantStateOfEnergy)
	eventID := fmt.Sprintf("audit-%d", now.UnixNano())
	stack.runEvent(t, ctx, eventID, now)
	if checked := stack.assertAuditChain(t, ctx, eventID); checked == 0 {
		t.Fatal("no command chains were checked")
	}
}

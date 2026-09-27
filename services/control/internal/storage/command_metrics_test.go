package storage

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/observability"
)

func metricValue(t *testing.T, prefix string) float64 {
	t.Helper()
	response := httptest.NewRecorder()
	observability.ProcessMetrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if response.Code != 200 {
		t.Fatalf("metrics status: %d", response.Code)
	}
	for line := range strings.SplitSeq(response.Body.String(), "\n") {
		if value, ok := strings.CutPrefix(line, prefix); ok {
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil {
				t.Fatal(err)
			}
			return parsed
		}
	}
	return 0
}

func commandMetricValue(t *testing.T, state string) float64 {
	return metricValue(t, `gridos_commands_total{state="`+state+`"} `)
}

func TestTransitionCommandRecordsCommittedStateOnce(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-metric-transition")
	command := testCommand("metric-transition", "event-metric-transition")
	if err := InsertCommand(context.Background(), pool, command); err != nil {
		t.Fatal(err)
	}
	before := commandMetricValue(t, "SENT")
	transition := CommandTransition{
		CommandID: command.CommandID, ExpectedState: "PERSISTED", NextState: "SENT",
		OccurredAt: time.Now().UTC(), CorrelationID: command.CorrelationID,
	}
	changed, err := TransitionCommand(context.Background(), pool, transition)
	if err != nil || !changed {
		t.Fatalf("first transition: changed=%t err=%v", changed, err)
	}
	if got := commandMetricValue(t, "SENT"); got != before+1 {
		t.Fatalf("SENT count = %v, want %v", got, before+1)
	}
	changed, err = TransitionCommand(context.Background(), pool, transition)
	if err != nil || changed {
		t.Fatalf("repeated transition: changed=%t err=%v", changed, err)
	}
	if got := commandMetricValue(t, "SENT"); got != before+1 {
		t.Fatalf("repeated SENT count = %v, want %v", got, before+1)
	}
}

func TestInsertCommandRecordsPersistedStateOnlyOnce(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-metric-insert")
	before := commandMetricValue(t, "PERSISTED")
	command := testCommand("metric-insert", "event-metric-insert")
	if err := InsertCommand(context.Background(), pool, command); err != nil {
		t.Fatal(err)
	}
	if got := commandMetricValue(t, "PERSISTED"); got != before+1 {
		t.Fatalf("insert count = %v, want %v", got, before+1)
	}
	if err := InsertCommand(context.Background(), pool, command); err == nil {
		t.Fatal("duplicate command inserted")
	}
	if got := commandMetricValue(t, "PERSISTED"); got != before+1 {
		t.Fatalf("failed insert count = %v, want %v", got, before+1)
	}
	zero := testCommand("metric-zero", "event-metric-insert")
	zero.SetpointKW = 0
	if err := InsertZeroCommand(context.Background(), pool, zero); err != nil {
		t.Fatal(err)
	}
	if got := commandMetricValue(t, "PERSISTED"); got != before+2 {
		t.Fatalf("zero insert count = %v, want %v", got, before+2)
	}
	if err := InsertZeroCommand(context.Background(), pool, zero); err != nil {
		t.Fatal(err)
	}
	if got := commandMetricValue(t, "PERSISTED"); got != before+2 {
		t.Fatalf("retried zero insert count = %v, want %v", got, before+2)
	}
}

func TestAcknowledgementRecordsCommittedState(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-metric-ack")
	command := testCommand("metric-ack", "event-metric-ack")
	if err := InsertCommand(context.Background(), pool, command); err != nil {
		t.Fatal(err)
	}
	_, err := TransitionCommand(context.Background(), pool, CommandTransition{
		CommandID: command.CommandID, ExpectedState: "PERSISTED", NextState: "SENT",
		OccurredAt: time.Now().UTC(), CorrelationID: command.CorrelationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	before := commandMetricValue(t, "ACKNOWLEDGED")
	ack := Acknowledgement{
		AcknowledgementID: "ack-metric", CommandID: command.CommandID,
		IdempotencyKey: "ack-metric-key", ReceiptStatus: "ACCEPTED",
		ReceivedAt: time.Now().UTC(), GatewayID: "gateway-metric", CorrelationID: command.CorrelationID,
	}
	if err := RecordAcknowledgement(context.Background(), pool, ack); err != nil {
		t.Fatal(err)
	}
	if got := commandMetricValue(t, "ACKNOWLEDGED"); got != before+1 {
		t.Fatalf("acknowledged count = %v, want %v", got, before+1)
	}
	if err := RecordAcknowledgement(context.Background(), pool, ack); err == nil {
		t.Fatal("repeated acknowledgement accepted")
	}
	if got := commandMetricValue(t, "ACKNOWLEDGED"); got != before+1 {
		t.Fatalf("repeated acknowledged count = %v, want %v", got, before+1)
	}
}

func TestUncertainCommandMetricFollowsDurableState(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-metric-uncertain")
	command := testCommand("metric-uncertain", "event-metric-uncertain")
	if err := InsertCommand(context.Background(), pool, command); err != nil {
		t.Fatal(err)
	}
	_, err := TransitionCommand(context.Background(), pool, CommandTransition{
		CommandID: command.CommandID, ExpectedState: "PERSISTED", NextState: "SENT",
		OccurredAt: time.Now().UTC(), CorrelationID: command.CorrelationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	before := commandMetricValue(t, "UNCERTAIN")
	now := time.Now().UTC()
	changed, err := MarkAcknowledgementUncertain(context.Background(), pool, now, now.Add(time.Second), testFeasiblePowerInterval(command, now))
	if err != nil || !changed {
		t.Fatalf("uncertain transition: changed=%t err=%v", changed, err)
	}
	if got := commandMetricValue(t, "UNCERTAIN"); got != before+1 {
		t.Fatalf("uncertain count = %v, want %v", got, before+1)
	}
	if got := metricValue(t, "gridos_uncertain_commands "); got != 1 {
		t.Fatalf("uncertain gauge = %v, want 1", got)
	}
	ack := Acknowledgement{
		AcknowledgementID: "ack-uncertain-metric", CommandID: command.CommandID,
		IdempotencyKey: "ack-uncertain-metric-key", ReceiptStatus: "ACCEPTED",
		ReceivedAt: now.Add(time.Second), GatewayID: "gateway-metric", CorrelationID: command.CorrelationID,
	}
	if err := RecordAcknowledgement(context.Background(), pool, ack); err != nil {
		t.Fatal(err)
	}
	if got := commandMetricValue(t, "UNCERTAIN"); got != before+1 {
		t.Fatalf("resolved uncertain count = %v, want %v", got, before+1)
	}
	if got := metricValue(t, "gridos_uncertain_commands "); got != 0 {
		t.Fatalf("resolved uncertain gauge = %v, want 0", got)
	}
}

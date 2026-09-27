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

func commandMetricValue(t *testing.T, state string) float64 {
	t.Helper()
	response := httptest.NewRecorder()
	observability.ProcessMetrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if response.Code != 200 {
		t.Fatalf("metrics status: %d", response.Code)
	}
	prefix := `gridos_commands_total{state="` + state + `"} `
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

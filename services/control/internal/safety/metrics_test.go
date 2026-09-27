package safety

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/observability"
)

func rejectionMetric(t *testing.T) float64 {
	t.Helper()
	response := httptest.NewRecorder()
	observability.ProcessMetrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	for line := range strings.SplitSeq(response.Body.String(), "\n") {
		if value, ok := strings.CutPrefix(line, "gridos_safety_rejections_total "); ok {
			count, err := strconv.ParseFloat(value, 64)
			if err != nil {
				t.Fatal(err)
			}
			return count
		}
	}
	t.Fatal("safety rejection metric absent")
	return 0
}

func TestValidateRecordsOneRejectionPerRejectedPlan(t *testing.T) {
	plan, state := validInputs()
	before := rejectionMetric(t)
	if approval, violations := Validate(plan, state); !approval.Approved || len(violations) != 0 {
		t.Fatalf("valid control rejected: %v", violations)
	}
	if got := rejectionMetric(t); got != before {
		t.Fatalf("valid plan changed rejection count: %v", got)
	}
	plan.TargetKW = 3
	plan.DeclaredShortfall = 0
	if approval, violations := Validate(plan, state); approval.Approved || len(violations) == 0 {
		t.Fatal("invalid plan was accepted")
	}
	if got := rejectionMetric(t); got != before+1 {
		t.Fatalf("rejection count = %v, want %v", got, before+1)
	}
}

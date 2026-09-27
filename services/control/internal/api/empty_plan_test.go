package api

import (
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/safety"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEmptyPlanShortfall(t *testing.T) {
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	canonical := safety.CanonicalState{Now: now, Boundary: safety.MeterNetExport, PolicyVersion: "policy-1", ExpectedGeneration: 1}
	shortfall := &gridosv1.ShortfallReport{
		IntervalBeginTime: timestamppb.New(now.Add(time.Minute)),
		IntervalEndTime:   timestamppb.New(now.Add(6 * time.Minute)),
		RequestedKw:       5,
		FeasibleKw:        0,
		ShortfallKw:       5,
	}
	plan := &gridosv1.DispatchPlan{Shortfalls: []*gridosv1.ShortfallReport{shortfall}}
	if err := (IndependentSafetyGate{}).Validate(plan, canonical); err != nil {
		t.Fatalf("full declared shortfall rejected: %v", err)
	}
	shortfall.ShortfallKw = 4
	if err := (IndependentSafetyGate{}).Validate(plan, canonical); err == nil {
		t.Fatal("underdeclared shortfall approved")
	}
}

package safety

import (
	"testing"
	"time"
)

func TestReapprovalRequiredForMaterialInputChange(t *testing.T) {
	plan, state := validInputs()
	approved, violations := Validate(plan, state)
	if !approved.Approved || len(violations) != 0 {
		t.Fatalf("baseline must be approved: %#v", violations)
	}
	current, violations := RevalidateApproval(plan, state, approved)
	if !current.Approved || len(violations) != 0 {
		t.Fatalf("unchanged inputs must retain approval: %#v", violations)
	}
	state.Now = state.Now.Add(time.Second)
	current, violations = RevalidateApproval(plan, state, approved)
	if !current.Approved || len(violations) != 0 {
		t.Fatalf("clock advancement alone must retain approval: %#v", violations)
	}
	device := state.Devices["device-1"]
	device.PlanReserveKWh = 9
	state.Devices["device-1"] = device
	current, violations = RevalidateApproval(plan, state, approved)
	if current.Approved || !violationCodes(violations)[ReapprovalRequired] {
		t.Fatalf("changed reserve must invalidate approval: %#v", violations)
	}
	newApproval, violations := Validate(plan, state)
	if !newApproval.Approved || len(violations) != 0 {
		t.Fatalf("new reserve remains physically feasible: %#v", violations)
	}
	current, violations = RevalidateApproval(plan, state, newApproval)
	if !current.Approved || len(violations) != 0 {
		t.Fatalf("new approval must bind changed inputs: %#v", violations)
	}
}

func TestReapprovalRequiredForChangedPlan(t *testing.T) {
	plan, state := validInputs()
	approved, _ := Validate(plan, state)
	plan.TargetKW = 1.5
	current, violations := RevalidateApproval(plan, state, approved)
	if current.Approved || !violationCodes(violations)[ReapprovalRequired] {
		t.Fatalf("changed plan must invalidate approval: %#v", violations)
	}
}

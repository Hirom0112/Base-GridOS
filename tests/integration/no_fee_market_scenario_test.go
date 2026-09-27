package integration

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNoFeeMarketReward(t *testing.T) {
	stack := startStack(t, "no-fee-market-reward")
	ctx := context.Background()
	cohort := stack.fleetDevices(t, 14)
	if len(cohort) == 0 {
		t.Fatal("market cohort is empty")
	}
	stack.publishTelemetry(t, ctx, cohort, time.Now().UTC(), constantStateOfEnergy)
	for _, device := range cohort {
		memberID := stack.selectScenarioPlan(t, device, 10, 10, 0)
		status := stack.memberStatus(t, memberID, device.SiteID)
		if status.GetCurrentPlan() == nil {
			t.Fatalf("no selected market plan for %s", memberID)
		}
	}
	var freePlans int
	if err := stack.pool.QueryRow(ctx, `SELECT count(*) FROM pricing_catalog_snapshots
		WHERE catalog_version LIKE 'scenario-catalog-%' AND energy_monthly_charge_cents = 0
		AND battery_monthly_charge_cents = 0`).Scan(&freePlans); err != nil {
		t.Fatal(err)
	}
	if freePlans != len(cohort) {
		t.Fatalf("no-fee selected catalog plans=%d want=%d", freePlans, len(cohort))
	}
	eventID := fmt.Sprintf("no-fee-reward-%d", time.Now().UnixNano())
	response := stack.runEvent(t, ctx, eventID, time.Now().UTC())
	stack.assertOutcome(t, ctx, eventID, response)
	var rewarded int
	var amount int64
	if err := stack.pool.QueryRow(ctx, `SELECT count(*), coalesce(sum(amount_cents), 0)::bigint
		FROM reward_ledger WHERE event_id = $1`, eventID).Scan(&rewarded, &amount); err != nil {
		t.Fatal(err)
	}
	var accepted int
	if err := stack.pool.QueryRow(ctx, `SELECT count(*) FROM command_acknowledgements acknowledgement
		JOIN command_intents intent USING (command_id) WHERE intent.event_id = $1
		AND intent.setpoint_kw <> 0 AND acknowledgement.receipt_status = 'ACCEPTED'`, eventID).Scan(&accepted); err != nil {
		t.Fatal(err)
	}
	report := stack.scenarioReport(t, eventID)
	if accepted == 0 || rewarded == 0 || rewarded > accepted || amount != int64(rewarded)*100 || report.MemberRewardsCents == nil || *report.MemberRewardsCents != amount {
		t.Fatalf("no-fee market accepted=%d reward rows=%d cents=%d stored report=%+v", accepted, rewarded, amount, report)
	}
}

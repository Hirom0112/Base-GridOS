package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type scenarioReport struct {
	EventID                    string
	PlanVersion                uint64
	ReserveViolationsPrevented uint64
	Versions                   struct{ Policy string }
	DataGaps                   []struct{ Reason string }
	Margin                     *struct {
		ValueUSD         float64
		Bound            string   `json:"margin_bound"`
		PriceProvenance  string   `json:"price_provenance"`
		UnavailableCosts []string `json:"unavailable_cost_terms"`
	}
}

func TestZeroPercentReservePreservesHardwareFloor(t *testing.T) {
	stack := startStack(t, "zero-percent-reserve-hardware-floor")
	ctx := context.Background()
	cohort := stack.cohort(t)
	var selected FleetDevice
	for _, device := range cohort {
		if device.ReservePreferencePercent == 0 {
			selected = device
			break
		}
	}
	if selected.DeviceID == "" {
		t.Fatal("cohort has no zero-percent member")
	}
	stack.publishTelemetry(t, ctx, cohort, time.Now().UTC(), constantStateOfEnergy)
	memberID := stack.selectScenarioPlan(t, selected, 0, 10, 0)
	status := stack.memberStatus(t, memberID, selected.SiteID)
	if status.GetEffectiveReservePercent() != 10 || status.GetCurrentPlan().GetReserveFloorPercent() != 0 {
		t.Fatalf("zero-percent member reserve=%g plan=%+v", status.GetEffectiveReservePercent(), status.GetCurrentPlan())
	}
	eventID := fmt.Sprintf("zero-reserve-%d", time.Now().UnixNano())
	response := stack.runEvent(t, ctx, eventID, time.Now().UTC())
	stack.assertOutcome(t, ctx, eventID, response)
	var frozen []byte
	if err := stack.pool.QueryRow(ctx, `SELECT inputs FROM input_snapshots WHERE event_id = $1 ORDER BY captured_at DESC LIMIT 1`, eventID).Scan(&frozen); err != nil {
		t.Fatal(err)
	}
	var snapshot gridosv1.OptimizationRequest
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(frozen, &snapshot); err != nil {
		t.Fatal(err)
	}
	for _, device := range snapshot.GetDevices() {
		if device.GetDeviceId() == selected.DeviceID {
			floor := device.GetUsableEnergyKwh() * 0.1
			if device.GetEffectiveReserveKwh() < floor || device.GetHardwareFloorKwh() < floor {
				t.Fatalf("zero-percent member frozen reserve=%g hardware=%g floor=%g", device.GetEffectiveReserveKwh(), device.GetHardwareFloorKwh(), floor)
			}
			report := stack.scenarioReport(t, eventID)
			if report.PlanVersion != 1 || report.ReserveViolationsPrevented != 0 || report.Versions.Policy == "" {
				t.Fatalf("stored report=%+v", report)
			}
			return
		}
	}
	t.Fatalf("frozen optimization omitted member device %s", selected.DeviceID)
}

func (stack *stack) selectScenarioPlan(t *testing.T, device FleetDevice, planFloor, hardwareFloor, monthlyCharge int64) string {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	memberID := "member-" + strings.TrimPrefix(device.SiteID, "site_")
	policyVersion := "scenario-policy-" + memberID
	catalogVersion := "scenario-catalog-" + memberID
	planID := "scenario-plan-" + memberID
	_, err := stack.pool.Exec(ctx, `INSERT INTO reserve_policies
		(policy_version, protected_hardware_floor_percent, member_plan_floor_percent, dynamic_override_percent,
		effective_reserve_percent, effective_at, correlation_id)
		VALUES ($1, $2, $3, 0, GREATEST($2::double precision, $3::double precision), $4, $5)`,
		policyVersion, hardwareFloor, planFloor, now.Add(-2*time.Hour), memberID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = stack.pool.Exec(ctx, `INSERT INTO pricing_catalog_snapshots
		(catalog_version, member_plan_id, market, display_name, reserve_floor_percent, energy_plan,
		energy_term_months, energy_monthly_charge_cents, battery_plan, battery_term_months,
		battery_monthly_charge_cents, flexibility_reward_cents, effective_at, correlation_id)
		VALUES ($1, $2, 'TX', 'Scenario plan', $3, '{}', 0, $4, '{}', 0, 0, 100, $5, $6)`,
		catalogVersion, planID, planFloor, monthlyCharge, now.Add(-2*time.Hour), memberID)
	if err != nil {
		t.Fatal(err)
	}
	client := gridosv1connect.NewMemberServiceClient(stack.client, stack.controlURL)
	offer := connect.NewRequest(&gridosv1.PresentOfferRequest{MemberId: memberID, IdempotencyKey: "plan-offer-" + memberID,
		Kind: gridosv1.MemberOfferKind_MEMBER_OFFER_KIND_PLAN, Market: "TX", CatalogVersion: catalogVersion,
		MemberPlanId: planID, ContractVersion: "scenario-contract-v1", PriceText: "Scenario catalog terms",
		ConsentText: "I accept the scenario plan", ConsentVersion: "scenario-consent-v1",
		EffectiveAt: timestamppb.New(now.Add(-time.Hour)), ExpiresAt: timestamppb.New(now.Add(time.Hour)), CorrelationId: memberID})
	memberHeaders(offer.Header(), memberID)
	if _, err = client.PresentOffer(ctx, offer); err != nil {
		t.Fatal(err)
	}
	selection := connect.NewRequest(&gridosv1.SelectResiliencePlanRequest{MemberId: memberID,
		IdempotencyKey: "plan-selection-" + memberID, OfferId: offer.Msg.IdempotencyKey,
		Market: "TX", CatalogVersion: catalogVersion, MemberPlanId: planID, PolicyVersion: policyVersion,
		ConsentText: offer.Msg.ConsentText, ConsentVersion: offer.Msg.ConsentVersion,
		ExplanationShown: "Backup reserve limits dispatch", EffectiveAt: timestamppb.New(now.Add(-time.Minute)), CorrelationId: memberID})
	memberHeaders(selection.Header(), memberID)
	if _, err = client.SelectResiliencePlan(ctx, selection); err != nil {
		t.Fatal(err)
	}
	return memberID
}

func (stack *stack) memberStatus(t *testing.T, memberID, siteID string) *gridosv1.GetMemberStatusResponse {
	t.Helper()
	request := connect.NewRequest(&gridosv1.GetMemberStatusRequest{MemberId: memberID, SiteId: siteID})
	memberHeaders(request.Header(), memberID)
	response, err := gridosv1connect.NewMemberServiceClient(stack.client, stack.controlURL).GetMemberStatus(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	return response.Msg
}

func (stack *stack) scenarioReport(t *testing.T, eventID string) scenarioReport {
	t.Helper()
	request := connect.NewRequest(&gridosv1.GetEventReportRequest{EventId: eventID})
	request.Header().Set("X-GridOS-Role", "analyst")
	response, err := gridosv1connect.NewReportServiceClient(&http.Client{Timeout: 30 * time.Second}, stack.controlURL).GetEventReport(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	var report scenarioReport
	if err = json.Unmarshal([]byte(response.Msg.GetReportJson()), &report); err != nil {
		t.Fatal(err)
	}
	if report.EventID != eventID {
		t.Fatalf("report event=%q, want %q", report.EventID, eventID)
	}
	return report
}

func memberHeaders(header http.Header, memberID string) {
	header.Set("X-GridOS-Role", "member")
	header.Set("X-GridOS-Member-ID", memberID)
}

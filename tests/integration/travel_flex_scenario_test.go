package integration

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTravelFlexLifecycle(t *testing.T) {
	stack := startStack(t, "travel-flex-lifecycle")
	ctx := context.Background()
	cohort := stack.cohort(t)
	index := slices.IndexFunc(cohort, func(device FleetDevice) bool { return device.ReservePreferencePercent <= 20 })
	if index < 0 {
		t.Fatal("cohort has no low-reserve member")
	}
	selected := cohort[index]
	stack.publishTelemetry(t, ctx, cohort, time.Now().UTC(), constantStateOfEnergy)
	memberID := stack.selectScenarioPlan(t, selected, 60, 10, 0)
	if got := stack.memberStatus(t, memberID, selected.SiteID).GetEffectiveReservePercent(); got != 60 {
		t.Fatalf("base reserve=%g, want 60", got)
	}
	start := time.Now().UTC().Add(-time.Second)
	end := start.Add(5 * time.Minute)
	windowID := stack.scheduleTravelFlex(t, memberID, start, end, "active")
	if got := stack.memberStatus(t, memberID, selected.SiteID).GetEffectiveReservePercent(); got != 20 {
		t.Fatalf("active Travel Flex reserve=%g, want 20", got)
	}
	eventID := fmt.Sprintf("travel-flex-%d", time.Now().UnixNano())
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
	var found bool
	for _, device := range snapshot.GetDevices() {
		if device.GetDeviceId() == selected.DeviceID {
			found = true
			floor := device.GetUsableEnergyKwh() * 0.2
			if device.GetTravelFlexReserveKwh() < floor || device.GetEffectiveReserveKwh() < floor {
				t.Fatalf("frozen Travel Flex reserve=%g effective=%g floor=%g", device.GetTravelFlexReserveKwh(), device.GetEffectiveReserveKwh(), floor)
			}
			break
		}
	}
	if !found {
		t.Fatalf("frozen inputs omit member device %s", selected.DeviceID)
	}
	report := stack.scenarioReport(t, eventID)
	if report.PlanVersion != 1 || report.ReserveViolationsPrevented != 0 {
		t.Fatalf("stored Travel Flex report=%+v", report)
	}
	request := connect.NewRequest(&gridosv1.EndTravelFlexEarlyRequest{MemberId: memberID, IdempotencyKey: "return-" + windowID,
		WindowId: windowID, ReturnedAt: timestamppb.Now(), ConsentVersion: "scenario-flex-consent-v1", CorrelationId: eventID})
	memberHeaders(request.Header(), memberID)
	if _, err := gridosv1connect.NewMemberServiceClient(stack.client, stack.controlURL).EndTravelFlexEarly(ctx, request); err != nil {
		t.Fatal(err)
	}
	if got := stack.memberStatus(t, memberID, selected.SiteID).GetEffectiveReservePercent(); got != 60 {
		t.Fatalf("early-return reserve=%g, want 60", got)
	}
	start = time.Now().UTC().Add(time.Second)
	end = start.Add(2 * time.Second)
	stack.scheduleTravelFlex(t, memberID, start, end, "expiry")
	time.Sleep(time.Until(start.Add(100 * time.Millisecond)))
	if got := stack.memberStatus(t, memberID, selected.SiteID).GetEffectiveReservePercent(); got != 20 {
		t.Fatalf("second window reserve=%g, want 20", got)
	}
	time.Sleep(time.Until(end.Add(100 * time.Millisecond)))
	if got := stack.memberStatus(t, memberID, selected.SiteID).GetEffectiveReservePercent(); got != 60 {
		t.Fatalf("expired reserve=%g, want 60", got)
	}
}

func (stack *stack) scheduleTravelFlex(t *testing.T, memberID string, start, end time.Time, suffix string) string {
	t.Helper()
	ctx := context.Background()
	client := gridosv1connect.NewMemberServiceClient(stack.client, stack.controlURL)
	offerID := "flex-offer-" + suffix + "-" + memberID
	reserve := float64(20)
	offer := connect.NewRequest(&gridosv1.PresentOfferRequest{MemberId: memberID, IdempotencyKey: offerID,
		Kind: gridosv1.MemberOfferKind_MEMBER_OFFER_KIND_TRAVEL_FLEX, Market: "TX", CatalogVersion: "scenario-catalog-" + memberID,
		MemberPlanId: "scenario-plan-" + memberID, ContractVersion: "scenario-flex-contract-v1", PriceText: "Fixed event credit",
		ConsentText: "I accept Travel Flex", ConsentVersion: "scenario-flex-consent-v1", EffectiveAt: timestamppb.New(start.Add(-time.Minute)),
		ExpiresAt: timestamppb.New(end), TemporaryReservePercent: &reserve,
		CreditType: gridosv1.MemberCreditType_MEMBER_CREDIT_TYPE_FIXED_EVENT, FixedCreditCents: 100,
		CorrelationId: offerID})
	memberHeaders(offer.Header(), memberID)
	if _, err := client.PresentOffer(ctx, offer); err != nil {
		t.Fatal(err)
	}
	windowID := "flex-window-" + suffix + "-" + memberID
	request := connect.NewRequest(&gridosv1.ScheduleTravelFlexRequest{MemberId: memberID, IdempotencyKey: windowID,
		OfferId: offerID, StartTime: timestamppb.New(start), EndTime: timestamppb.New(end), Timezone: "America/Chicago",
		TemporaryReservePercent: reserve, EarlyReturnAction: gridosv1.MemberEarlyReturnAction_MEMBER_EARLY_RETURN_ACTION_RESTORE_PLAN_RESERVE,
		CreditType: gridosv1.MemberCreditType_MEMBER_CREDIT_TYPE_FIXED_EVENT, FixedCreditCents: 100,
		ConsentText: offer.Msg.ConsentText, ConsentVersion: offer.Msg.ConsentVersion,
		PolicyVersion: "scenario-policy-" + memberID, CorrelationId: windowID})
	memberHeaders(request.Header(), memberID)
	response, err := client.ScheduleTravelFlex(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	return response.Msg.GetWindowId()
}

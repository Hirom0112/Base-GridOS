package member

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestListMemberOffersUsesStoredTerms(t *testing.T) {
	ctx := context.Background()
	pool := memberDatabase(t)
	now := time.Now().UTC().Truncate(time.Second)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id, member_id, bound_at, source, provenance) VALUES ('site-offers', 'member-offers', now(), 'SIMULATED', '{"provenance":"SIMULATED"}')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO pricing_catalog_snapshots(catalog_version,member_plan_id,market,display_name,reserve_floor_percent,energy_plan,energy_term_months,energy_monthly_charge_cents,battery_plan,battery_term_months,battery_monthly_charge_cents,flexibility_reward_cents,effective_at,expires_at,correlation_id) VALUES ('catalog-offers','balanced','ERCOT','Balanced',30,'{}',0,1200,'{}',0,0,800,$1,$2,'fixture')`, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO offer_terms(catalog_version,member_plan_id,kind,contract_version,consent_version,consent_text,price_text,temporary_reserve_percent,credit_type,fixed_credit_cents) VALUES ('catalog-offers','balanced','PLAN','contract-1','consent-1','Stored consent','Stored price',NULL,NULL,0),('catalog-offers','balanced','TRAVEL_FLEX','contract-1','consent-flex-1','Stored flex consent','Stored flex price',20,'FIXED_DAILY',500)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO reserve_policies(policy_version,protected_hardware_floor_percent,member_plan_floor_percent,dynamic_override_percent,effective_reserve_percent,effective_at,correlation_id) VALUES ('policy-offers',10,30,0,30,$1,'fixture')`, now.Add(-time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO travel_flex_windows(travel_flex_window_id,member_id,start_time,end_time,timezone,temporary_reserve_percent,early_return_action,credit_type,credit_cents,consent_text,consent_version,policy_version,correlation_id) VALUES ('travel-offers','member-offers',$1,$2,'UTC',20,'RESTORE_PLAN_RESERVE','FIXED_DAILY',500,'Stored flex consent','consent-flex-1','policy-offers','fixture')`, now.Add(time.Hour), now.Add(2*time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO member_away_periods(away_period_id,member_id,start_time,end_time,consent_version,correlation_id) VALUES ('away-offers','member-offers',$1,$2,'away-consent','fixture')`, now.Add(time.Hour), now.Add(2*time.Hour))
	require.NoError(t, err)
	service := NewService(pool, fleet.NewTwin(time.Minute), nil, func() time.Time { return now })
	_, handler := gridosv1connect.NewMemberServiceHandler(service)
	server := httptest.NewServer(handler)
	defer server.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/gridos.v1.MemberService/ListMemberOffers", bytes.NewBufferString(`{"memberId":"member-offers"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GridOS-Role", "member")
	request.Header.Set("X-GridOS-Member-ID", "member-offers")
	response, err := server.Client().Do(request)
	require.NoError(t, err)
	defer func() { require.NoError(t, response.Body.Close()) }()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("ListMemberOffers status = %d: %s", response.StatusCode, body)
	}
	var listed struct {
		Offers []struct {
			Kind           string `json:"kind"`
			PriceText      string `json:"priceText"`
			ConsentVersion string `json:"consentVersion"`
		} `json:"offers"`
		TravelFlexWindows []json.RawMessage `json:"travelFlexWindows"`
		AwayWindows       []json.RawMessage `json:"awayWindows"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Offers) != 2 || listed.Offers[0].PriceText != "Stored price" || listed.Offers[1].ConsentVersion != "consent-flex-1" || len(listed.TravelFlexWindows) != 1 || len(listed.AwayWindows) != 1 {
		t.Fatalf("stored offers and windows = %s", body)
	}
	offer := connect.NewRequest(&gridosv1.PresentOfferRequest{MemberId: "member-offers", IdempotencyKey: "offer-forged", Kind: gridosv1.MemberOfferKind_MEMBER_OFFER_KIND_PLAN, Market: "ERCOT", CatalogVersion: "catalog-offers", MemberPlanId: "balanced", ContractVersion: "contract-1", PriceText: "Forged price", ConsentText: "Stored consent", ConsentVersion: "consent-1", EffectiveAt: timestamppb.New(now), ExpiresAt: timestamppb.New(now.Add(time.Hour)), CorrelationId: "fixture"})
	offer.Header().Set("X-GridOS-Role", "member")
	offer.Header().Set("X-GridOS-Member-ID", "member-offers")
	_, err = service.PresentOffer(ctx, offer)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("forged terms code = %v", connect.CodeOf(err))
	}
	offer.Msg.PriceText = "Stored price"
	offer.Msg.IdempotencyKey = "offer-valid"
	accepted, err := service.PresentOffer(ctx, offer)
	if err != nil || accepted.Msg.GetOffer().GetPriceText() != "Stored price" {
		t.Fatalf("stored terms rejected: %v, %#v", err, accepted)
	}
}

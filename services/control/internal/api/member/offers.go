package member

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/policy"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) ListMemberOffers(ctx context.Context, request *connect.Request[gridosv1.ListMemberOffersRequest]) (*connect.Response[gridosv1.ListMemberOffersResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	memberID := request.Msg.GetMemberId()
	if err := service.authorize(ctx, request.Header().Get("X-GridOS-Role"), request.Header().Get("X-GridOS-Member-ID"), memberID, "", true); err != nil {
		return nil, err
	}
	now := service.now()
	offers, err := service.catalogOffers(ctx, now)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	travel, err := service.scheduledTravelFlex(ctx, memberID, now)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	away, err := service.scheduledAway(ctx, memberID, now)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&gridosv1.ListMemberOffersResponse{Offers: offers, TravelFlexWindows: travel, AwayWindows: away}), nil
}

func (service *Service) catalogOffers(ctx context.Context, now time.Time) ([]*gridosv1.MemberOfferTerms, error) {
	rows, err := service.pool.Query(ctx, `SELECT catalog.market, catalog.catalog_version, catalog.member_plan_id, catalog.display_name,
  catalog.reserve_floor_percent, catalog.energy_monthly_charge_cents, catalog.battery_monthly_charge_cents,
  catalog.flexibility_reward_cents, catalog.effective_at, catalog.expires_at, terms.kind, terms.policy_version, terms.contract_version,
  terms.consent_version, terms.consent_text, terms.price_text, terms.temporary_reserve_percent,
  terms.credit_type, terms.fixed_credit_cents FROM pricing_catalog_snapshots catalog
  JOIN offer_terms terms USING (catalog_version, member_plan_id)
  WHERE catalog.effective_at <= $1 AND (catalog.expires_at IS NULL OR catalog.expires_at > $1)
  ORDER BY catalog.market, catalog.member_plan_id, terms.kind`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var offers []*gridosv1.MemberOfferTerms
	for rows.Next() {
		offer := &gridosv1.MemberOfferTerms{}
		var kind string
		var credit *string
		var expires *time.Time
		var effective time.Time
		err = rows.Scan(&offer.Market, &offer.CatalogVersion, &offer.MemberPlanId, &offer.DisplayName,
			&offer.ReserveFloorPercent, &offer.EnergyMonthlyChargeCents, &offer.BatteryMonthlyChargeCents,
			&offer.FlexibilityRewardCents, &effective, &expires, &kind, &offer.PolicyVersion, &offer.ContractVersion,
			&offer.ConsentVersion, &offer.ConsentText, &offer.PriceText, &offer.TemporaryReservePercent,
			&credit, &offer.FixedCreditCents)
		if err != nil {
			return nil, err
		}
		offer.EffectiveAt = timestamppb.New(effective)
		if expires != nil {
			offer.ExpiresAt = timestamppb.New(*expires)
		}
		switch kind {
		case string(policy.PlanOffer):
			offer.Kind = gridosv1.MemberOfferKind_MEMBER_OFFER_KIND_PLAN
		case string(policy.TravelFlexOffer):
			offer.Kind = gridosv1.MemberOfferKind_MEMBER_OFFER_KIND_TRAVEL_FLEX
			offer.FlexibilityRewardCents = offer.FixedCreditCents
		default:
			return nil, errors.New("unknown stored offer kind")
		}
		if credit != nil {
			offer.CreditType, err = memberCreditType(*credit)
			if err != nil {
				return nil, err
			}
		}
		offers = append(offers, offer)
	}
	return offers, rows.Err()
}

func memberCreditType(value string) (gridosv1.MemberCreditType, error) {
	switch policy.CreditType(value) {
	case policy.FixedDaily:
		return gridosv1.MemberCreditType_MEMBER_CREDIT_TYPE_FIXED_DAILY, nil
	case policy.FixedEvent:
		return gridosv1.MemberCreditType_MEMBER_CREDIT_TYPE_FIXED_EVENT, nil
	case policy.FixedAnnual:
		return gridosv1.MemberCreditType_MEMBER_CREDIT_TYPE_FIXED_ANNUAL, nil
	default:
		return 0, errors.New("unknown stored credit type")
	}
}

func (service *Service) scheduledTravelFlex(ctx context.Context, memberID string, now time.Time) ([]*gridosv1.ScheduledTravelFlexWindow, error) {
	rows, err := service.pool.Query(ctx, `SELECT travel_flex_window_id, start_time, end_time, timezone,
  temporary_reserve_percent, credit_type, credit_cents, cancelled_at, consent_version, early_return_action
  FROM travel_flex_windows WHERE member_id = $1 AND end_time > $2 ORDER BY start_time LIMIT 100`, memberID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var windows []*gridosv1.ScheduledTravelFlexWindow
	for rows.Next() {
		window := &gridosv1.ScheduledTravelFlexWindow{}
		var start, end time.Time
		var cancelled *time.Time
		var credit string
		var earlyReturn string
		if err := rows.Scan(&window.WindowId, &start, &end, &window.Timezone, &window.TemporaryReservePercent,
			&credit, &window.FixedCreditCents, &cancelled, &window.ConsentVersion, &earlyReturn); err != nil {
			return nil, err
		}
		window.StartTime, window.EndTime = timestamppb.New(start), timestamppb.New(end)
		if cancelled != nil {
			window.CancelledAt = timestamppb.New(*cancelled)
		}
		window.CreditType, err = memberCreditType(credit)
		if err != nil {
			return nil, err
		}
		switch policy.EarlyReturnAction(earlyReturn) {
		case policy.RestorePlanReserve:
			window.EarlyReturnAction = gridosv1.MemberEarlyReturnAction_MEMBER_EARLY_RETURN_ACTION_RESTORE_PLAN_RESERVE
		case policy.RestoreMaximumReserve:
			window.EarlyReturnAction = gridosv1.MemberEarlyReturnAction_MEMBER_EARLY_RETURN_ACTION_RESTORE_MAXIMUM_RESERVE
		default:
			return nil, errors.New("unknown stored early return action")
		}
		windows = append(windows, window)
	}
	return windows, rows.Err()
}

func (service *Service) scheduledAway(ctx context.Context, memberID string, now time.Time) ([]*gridosv1.ScheduledAwayWindow, error) {
	rows, err := service.pool.Query(ctx, `SELECT away_period_id, start_time, end_time, ended_at, consent_version
  FROM member_away_periods WHERE member_id = $1 AND end_time > $2 ORDER BY start_time LIMIT 100`, memberID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var windows []*gridosv1.ScheduledAwayWindow
	for rows.Next() {
		window := &gridosv1.ScheduledAwayWindow{}
		var start, end time.Time
		var ended *time.Time
		if err := rows.Scan(&window.WindowId, &start, &end, &ended, &window.ConsentVersion); err != nil {
			return nil, err
		}
		window.StartTime, window.EndTime = timestamppb.New(start), timestamppb.New(end)
		if ended != nil {
			window.EndedAt = timestamppb.New(*ended)
		}
		windows = append(windows, window)
	}
	return windows, rows.Err()
}

func (service *Service) validateOfferTerms(ctx context.Context, offer policy.Offer) error {
	var contract, consentVersion, consentText, priceText string
	var reserve *float64
	var credit *string
	var cents int64
	err := service.pool.QueryRow(ctx, `SELECT contract_version, consent_version, consent_text, price_text,
  temporary_reserve_percent, credit_type, fixed_credit_cents FROM offer_terms
  WHERE catalog_version = $1 AND member_plan_id = $2 AND kind = $3`, offer.CatalogVersion, offer.MemberPlanID, offer.Kind).Scan(
		&contract, &consentVersion, &consentText, &priceText, &reserve, &credit, &cents)
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("stored offer terms not found"))
	}
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if offer.ContractVersion != contract || offer.ConsentVersion != consentVersion || offer.ConsentText != consentText || offer.PriceText != priceText || offer.CreditCents != cents || (offer.TemporaryReservePercent == nil) != (reserve == nil) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("submitted offer terms differ from stored catalog"))
	}
	if reserve != nil && *offer.TemporaryReservePercent != *reserve || (credit == nil && offer.CreditType != "") || (credit != nil && string(offer.CreditType) != *credit) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("submitted offer terms differ from stored catalog"))
	}
	return nil
}

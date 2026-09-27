package member

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) commandContext(ctx context.Context, role, principal, memberID, key, consentVersion string) (context.Context, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := service.authorize(ctx, role, principal, memberID, "", false); err != nil {
		cancel()
		return nil, nil, err
	}
	if key == "" || consentVersion == "" {
		cancel()
		return nil, nil, connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency key and consent version required"))
	}
	return ctx, cancel, nil
}

func timestamp(value *timestamppb.Timestamp) (time.Time, error) {
	if value == nil || value.CheckValid() != nil {
		return time.Time{}, connect.NewError(connect.CodeInvalidArgument, errors.New("valid timestamp required"))
	}
	return value.AsTime().UTC(), nil
}

func policyError(err error) error {
	if err == nil {
		return nil
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		return connect.NewError(connect.CodeInternal, errors.New("member policy storage failed"))
	}
	return connect.NewError(connect.CodeFailedPrecondition, err)
}

func offerKind(value gridosv1.MemberOfferKind) policy.OfferKind {
	switch value {
	case gridosv1.MemberOfferKind_MEMBER_OFFER_KIND_PLAN:
		return policy.PlanOffer
	case gridosv1.MemberOfferKind_MEMBER_OFFER_KIND_TRAVEL_FLEX:
		return policy.TravelFlexOffer
	default:
		return ""
	}
}

func creditType(value gridosv1.MemberCreditType) policy.CreditType {
	switch value {
	case gridosv1.MemberCreditType_MEMBER_CREDIT_TYPE_FIXED_DAILY:
		return policy.FixedDaily
	case gridosv1.MemberCreditType_MEMBER_CREDIT_TYPE_FIXED_EVENT:
		return policy.FixedEvent
	case gridosv1.MemberCreditType_MEMBER_CREDIT_TYPE_FIXED_ANNUAL:
		return policy.FixedAnnual
	default:
		return ""
	}
}

func earlyReturnAction(value gridosv1.MemberEarlyReturnAction) policy.EarlyReturnAction {
	switch value {
	case gridosv1.MemberEarlyReturnAction_MEMBER_EARLY_RETURN_ACTION_RESTORE_PLAN_RESERVE:
		return policy.RestorePlanReserve
	case gridosv1.MemberEarlyReturnAction_MEMBER_EARLY_RETURN_ACTION_RESTORE_MAXIMUM_RESERVE:
		return policy.RestoreMaximumReserve
	default:
		return ""
	}
}

func (service *Service) PresentOffer(ctx context.Context, request *connect.Request[gridosv1.PresentOfferRequest]) (*connect.Response[gridosv1.PresentOfferResponse], error) {
	msg := request.Msg
	ctx, cancel, err := service.commandContext(ctx, request.Header().Get("X-GridOS-Role"), request.Header().Get("X-GridOS-Member-ID"), msg.GetMemberId(), msg.GetIdempotencyKey(), msg.GetConsentVersion())
	if err != nil {
		return nil, err
	}
	defer cancel()
	effective, err := timestamp(msg.GetEffectiveAt())
	if err != nil {
		return nil, err
	}
	expires, err := timestamp(msg.GetExpiresAt())
	if err != nil {
		return nil, err
	}
	offer := policy.Offer{ID: msg.GetIdempotencyKey(), MemberID: msg.GetMemberId(), Kind: offerKind(msg.GetKind()),
		Market: msg.GetMarket(), CatalogVersion: msg.GetCatalogVersion(), MemberPlanID: msg.GetMemberPlanId(),
		ContractVersion: msg.GetContractVersion(), PriceText: msg.GetPriceText(), ConsentText: msg.GetConsentText(),
		ConsentVersion: msg.GetConsentVersion(), EffectiveAt: effective, ExpiresAt: expires,
		CreditType: creditType(msg.GetCreditType()), CreditCents: msg.GetFixedCreditCents(), CorrelationID: msg.GetCorrelationId()}
	if msg.TemporaryReservePercent != nil {
		value := msg.GetTemporaryReservePercent()
		offer.TemporaryReservePercent = &value
	}
	if err := service.validateOfferTerms(ctx, offer); err != nil {
		return nil, err
	}
	stored, err := service.policy.PresentOffer(ctx, offer)
	if err != nil {
		return nil, policyError(err)
	}
	result := &gridosv1.MemberOffer{OfferId: stored.ID, MemberId: stored.MemberID, Kind: msg.GetKind(),
		Market: stored.Market, CatalogVersion: stored.CatalogVersion, MemberPlanId: stored.MemberPlanID,
		ContractVersion: stored.ContractVersion, PriceText: stored.PriceText, ConsentText: stored.ConsentText,
		ConsentVersion: stored.ConsentVersion, EffectiveAt: timestamppb.New(stored.EffectiveAt),
		ExpiresAt: timestamppb.New(stored.ExpiresAt), CreditType: msg.GetCreditType(),
		FixedCreditCents: stored.CreditCents, EnergyMonthlyChargeCents: stored.EnergyMonthlyChargeCents,
		BatteryMonthlyChargeCents: stored.BatteryMonthlyChargeCents, FlexibilityRewardCents: stored.FlexibilityRewardCents}
	result.TemporaryReservePercent = stored.TemporaryReservePercent
	return connect.NewResponse(&gridosv1.PresentOfferResponse{Offer: result}), nil
}

func (service *Service) SelectResiliencePlan(ctx context.Context, request *connect.Request[gridosv1.SelectResiliencePlanRequest]) (*connect.Response[gridosv1.SelectResiliencePlanResponse], error) {
	msg := request.Msg
	ctx, cancel, err := service.commandContext(ctx, request.Header().Get("X-GridOS-Role"), request.Header().Get("X-GridOS-Member-ID"), msg.GetMemberId(), msg.GetIdempotencyKey(), msg.GetConsentVersion())
	if err != nil {
		return nil, err
	}
	defer cancel()
	effective, err := timestamp(msg.GetEffectiveAt())
	if err != nil {
		return nil, err
	}
	plan, err := service.policy.Select(ctx, policy.Selection{ID: msg.GetIdempotencyKey(), OfferID: msg.GetOfferId(),
		MemberID: msg.GetMemberId(), Market: msg.GetMarket(), CatalogVersion: msg.GetCatalogVersion(),
		MemberPlanID: msg.GetMemberPlanId(), PolicyVersion: msg.GetPolicyVersion(), ConsentText: msg.GetConsentText(),
		ConsentVersion: msg.GetConsentVersion(), ExplanationShown: msg.GetExplanationShown(),
		EffectiveAt: effective, CorrelationID: msg.GetCorrelationId()})
	if err != nil {
		return nil, policyError(err)
	}
	return connect.NewResponse(&gridosv1.SelectResiliencePlanResponse{Plan: selectedPlan(plan)}), nil
}

func (service *Service) ScheduleTravelFlex(ctx context.Context, request *connect.Request[gridosv1.ScheduleTravelFlexRequest]) (*connect.Response[gridosv1.ScheduleTravelFlexResponse], error) {
	msg := request.Msg
	ctx, cancel, err := service.commandContext(ctx, request.Header().Get("X-GridOS-Role"), request.Header().Get("X-GridOS-Member-ID"), msg.GetMemberId(), msg.GetIdempotencyKey(), msg.GetConsentVersion())
	if err != nil {
		return nil, err
	}
	defer cancel()
	start, err := timestamp(msg.GetStartTime())
	if err != nil {
		return nil, err
	}
	end, err := timestamp(msg.GetEndTime())
	if err != nil {
		return nil, err
	}
	zone, err := time.LoadLocation(msg.GetTimezone())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	window, err := service.policy.ScheduleTravelFlex(ctx, policy.TravelFlex{ID: msg.GetIdempotencyKey(), OfferID: msg.GetOfferId(),
		MemberID: msg.GetMemberId(), Start: start.In(zone), End: end.In(zone), Timezone: msg.GetTimezone(),
		TemporaryReservePercent: msg.GetTemporaryReservePercent(), EarlyReturnAction: earlyReturnAction(msg.GetEarlyReturnAction()),
		CreditType: creditType(msg.GetCreditType()), CreditCents: msg.GetFixedCreditCents(),
		ConsentText: msg.GetConsentText(), ConsentVersion: msg.GetConsentVersion(),
		PolicyVersion: msg.GetPolicyVersion(), CorrelationID: msg.GetCorrelationId()})
	if err != nil {
		return nil, policyError(err)
	}
	return connect.NewResponse(&gridosv1.ScheduleTravelFlexResponse{WindowId: window.ID,
		StartTime: timestamppb.New(window.Start), EndTime: timestamppb.New(window.End),
		TemporaryReservePercent: window.TemporaryReservePercent, FixedCreditCents: window.CreditCents}), nil
}

func (service *Service) EndTravelFlexEarly(ctx context.Context, request *connect.Request[gridosv1.EndTravelFlexEarlyRequest]) (*connect.Response[gridosv1.EndTravelFlexEarlyResponse], error) {
	msg := request.Msg
	ctx, cancel, err := service.commandContext(ctx, request.Header().Get("X-GridOS-Role"), request.Header().Get("X-GridOS-Member-ID"), msg.GetMemberId(), msg.GetIdempotencyKey(), msg.GetConsentVersion())
	if err != nil {
		return nil, err
	}
	defer cancel()
	returned, err := timestamp(msg.GetReturnedAt())
	if err != nil {
		return nil, err
	}
	if err := service.requireConsent(ctx, `SELECT consent_version FROM travel_flex_windows WHERE member_id = $1 AND travel_flex_window_id = $2`, msg.GetMemberId(), msg.GetWindowId(), msg.GetConsentVersion()); err != nil {
		return nil, err
	}
	err = service.policy.EndTravelFlexEarly(ctx, policy.EarlyReturn{ID: msg.GetIdempotencyKey(), WindowID: msg.GetWindowId(),
		MemberID: msg.GetMemberId(), At: returned, CorrelationID: msg.GetCorrelationId()})
	if err != nil {
		return nil, policyError(err)
	}
	return connect.NewResponse(&gridosv1.EndTravelFlexEarlyResponse{WindowId: msg.GetWindowId(), ReturnedAt: timestamppb.New(returned)}), nil
}

func (service *Service) requireConsent(ctx context.Context, query, memberID, resourceID, version string) error {
	var stored string
	err := service.pool.QueryRow(ctx, query, memberID, resourceID).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewError(connect.CodeNotFound, errors.New("member resource not found"))
	}
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if stored != version {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("consent version does not match"))
	}
	return nil
}

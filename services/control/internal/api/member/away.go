package member

import (
	"context"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/policy"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) SetAnomalyPreference(ctx context.Context, request *connect.Request[gridosv1.SetAnomalyPreferenceRequest]) (*connect.Response[gridosv1.SetAnomalyPreferenceResponse], error) {
	msg := request.Msg
	ctx, cancel, err := service.commandContext(ctx, request.Header().Get("X-GridOS-Role"), request.Header().Get("X-GridOS-Member-ID"), msg.GetMemberId(), msg.GetIdempotencyKey(), msg.GetConsentVersion())
	if err != nil {
		return nil, err
	}
	defer cancel()
	begin, err := timestamp(msg.GetBaselineBegin())
	if err != nil {
		return nil, err
	}
	end, err := timestamp(msg.GetBaselineEnd())
	if err != nil {
		return nil, err
	}
	effective, err := timestamp(msg.GetEffectiveAt())
	if err != nil {
		return nil, err
	}
	expires, err := timestamp(msg.GetExpiresAt())
	if err != nil {
		return nil, err
	}
	err = service.policy.SetAnomalyPreference(ctx, policy.AnomalyPreference{ID: msg.GetIdempotencyKey(),
		MemberID: msg.GetMemberId(), OptIn: msg.GetOptedIn(), ConsentText: msg.GetConsentText(),
		ConsentVersion: msg.GetConsentVersion(), BaselineUpperKW: msg.GetBaselineUpperKw(),
		BaselineBegin: begin, BaselineEnd: end, EffectiveAt: effective, ExpiresAt: expires,
		CorrelationID: msg.GetCorrelationId()})
	if err != nil {
		return nil, policyError(err)
	}
	return connect.NewResponse(&gridosv1.SetAnomalyPreferenceResponse{PreferenceId: msg.GetIdempotencyKey()}), nil
}

func (service *Service) ScheduleAway(ctx context.Context, request *connect.Request[gridosv1.ScheduleAwayRequest]) (*connect.Response[gridosv1.ScheduleAwayResponse], error) {
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
	err = service.policy.ScheduleAway(ctx, policy.AwayPeriod{ID: msg.GetIdempotencyKey(), MemberID: msg.GetMemberId(),
		Start: start, End: end, ConsentVersion: msg.GetConsentVersion(), CorrelationID: msg.GetCorrelationId()})
	if err != nil {
		return nil, policyError(err)
	}
	return connect.NewResponse(&gridosv1.ScheduleAwayResponse{AwayPeriodId: msg.GetIdempotencyKey()}), nil
}

func (service *Service) EndAway(ctx context.Context, request *connect.Request[gridosv1.EndAwayRequest]) (*connect.Response[gridosv1.EndAwayResponse], error) {
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
	if err := service.requireConsent(ctx, `SELECT consent_version FROM member_away_periods WHERE member_id = $1 AND away_period_id = $2`, msg.GetMemberId(), msg.GetAwayPeriodId(), msg.GetConsentVersion()); err != nil {
		return nil, err
	}
	err = service.policy.EndAway(ctx, policy.EndAway{ID: msg.GetIdempotencyKey(), PeriodID: msg.GetAwayPeriodId(),
		MemberID: msg.GetMemberId(), At: returned, CorrelationID: msg.GetCorrelationId()})
	if err != nil {
		return nil, policyError(err)
	}
	return connect.NewResponse(&gridosv1.EndAwayResponse{AwayPeriodId: msg.GetAwayPeriodId(), ReturnedAt: timestamppb.New(returned)}), nil
}

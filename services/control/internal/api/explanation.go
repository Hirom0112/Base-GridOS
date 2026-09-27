package api

import (
	"context"
	"errors"
	"math"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
)

func (service *Service) GetPlanExplanation(ctx context.Context, request *connect.Request[gridosv1.GetPlanExplanationRequest]) (*connect.Response[gridosv1.GetPlanExplanationResponse], error) {
	if err := authorize(request.Header(), "operator", "approver", "analyst", "service"); err != nil {
		return nil, err
	}
	if request.Msg.GetEventId() == "" || request.Msg.GetPlanVersion() == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event ID and plan version required"))
	}
	event, _, err := service.store.Get(ctx, request.Msg.GetEventId())
	if err != nil {
		return nil, storeError(err)
	}
	if event.GetPlanVersion() != request.Msg.GetPlanVersion() {
		return nil, storeError(ErrPlanVersion)
	}
	input, plan, err := service.store.LoadPlan(ctx, event.GetEventId(), event.GetPlanVersion())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if input == nil || plan == nil || plan.GetEventId() != event.GetEventId() || plan.GetPlanVersion() != event.GetPlanVersion() {
		return nil, connect.NewError(connect.CodeInternal, errors.New("stored plan does not match event"))
	}
	reserve := 0.0
	for _, device := range input.GetDevices() {
		reserve += device.GetEffectiveReserveKwh()
	}
	if math.IsNaN(reserve) || math.IsInf(reserve, 0) {
		return nil, connect.NewError(connect.CodeInternal, errors.New("stored reserve is not finite"))
	}
	return connect.NewResponse(&gridosv1.GetPlanExplanationResponse{
		ObjectiveBreakdown: plan.GetObjectiveBreakdown(), ReserveHeldBackKwh: reserve,
		ConstraintMargins: plan.GetConstraintMargins(), Exclusions: plan.GetExclusions(), Shortfalls: plan.GetShortfalls(),
		MarginExplanation: plan.GetMarginExplanation(), Evidence: &gridosv1.PlanExplanationEvidence{
			SiteLoads: input.GetForecast().GetSiteLoads(), SiteLoadUnits: "kWh", FallbackUsed: plan.GetFallbackUsed(),
			FallbackReason: plan.GetFallbackReason(), DeviceSchedules: plan.GetDeviceSchedules(),
			RegionalPrices: input.GetForecast().GetRegionalPrices(), OutageRisks: input.GetForecast().GetOutageRisks(),
			DeviceAvailability: input.GetForecast().GetDeviceAvailability(), UnavailableSources: input.GetForecast().GetUnavailableSources(),
		},
	}), nil
}

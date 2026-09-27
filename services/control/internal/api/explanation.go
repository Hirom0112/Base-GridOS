package api

import (
	"context"
	"errors"
	"math"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/proto"
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
	reserve, bases, err := planReserveEvidence(input, plan)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&gridosv1.GetPlanExplanationResponse{
		ObjectiveBreakdown: plan.GetObjectiveBreakdown(), ReserveHeldBackKwh: reserve,
		ConstraintMargins: plan.GetConstraintMargins(), Exclusions: plan.GetExclusions(), Shortfalls: plan.GetShortfalls(),
		MarginExplanation: plan.GetMarginExplanation(), Evidence: &gridosv1.PlanExplanationEvidence{
			SiteLoads: input.GetForecast().GetSiteLoads(), SiteLoadUnits: "kWh", FallbackUsed: plan.GetFallbackUsed(),
			FallbackReason: plan.GetFallbackReason(), DeviceSchedules: plan.GetDeviceSchedules(),
			RegionalPrices: input.GetForecast().GetRegionalPrices(), OutageRisks: input.GetForecast().GetOutageRisks(),
			DeviceAvailability: input.GetForecast().GetDeviceAvailability(), UnavailableSources: input.GetForecast().GetUnavailableSources(),
			ReserveBases: bases, TravelFlexBindings: input.GetEligibilitySnapshot().GetTravelFlexBindings(),
		},
	}), nil
}

func planReserveEvidence(input *gridosv1.OptimizationRequest, plan *gridosv1.DispatchPlan) (float64, []*gridosv1.FrozenReserveBasis, error) {
	selectedReserve := make(map[string]float64, len(plan.GetDeviceSchedules()))
	for _, schedule := range plan.GetDeviceSchedules() {
		if schedule.GetReserveSelection() != gridosv1.ReserveSelection_RESERVE_SELECTION_UNSPECIFIED {
			selectedReserve[schedule.GetDeviceId()] = schedule.GetSelectedReserveKwh()
		}
	}
	reserve := 0.0
	for _, device := range input.GetDevices() {
		value, selected := selectedReserve[device.GetDeviceId()]
		if !selected {
			value = device.GetEffectiveReserveKwh()
		}
		reserve += value
	}
	if math.IsNaN(reserve) || math.IsInf(reserve, 0) {
		return 0, nil, errors.New("stored reserve is not finite")
	}
	bases := make([]*gridosv1.FrozenReserveBasis, 0, len(input.GetEligibilitySnapshot().GetReserveBases()))
	for _, frozen := range input.GetEligibilitySnapshot().GetReserveBases() {
		basis := proto.Clone(frozen).(*gridosv1.FrozenReserveBasis)
		if value, selected := selectedReserve[basis.GetDeviceId()]; selected {
			basis.EffectiveReserveKwh = value
		}
		bases = append(bases, basis)
	}
	return reserve, bases, nil
}

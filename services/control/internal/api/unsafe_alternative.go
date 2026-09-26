package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/safety"
)

func (service *Service) ValidateUnsafeAlternative(ctx context.Context, request *connect.Request[gridosv1.ValidateUnsafeAlternativeRequest]) (*connect.Response[gridosv1.ValidateUnsafeAlternativeResponse], error) {
	if err := authorize(request.Header(), "operator", "approver"); err != nil {
		return nil, err
	}
	alternative := request.Msg.GetAlternativePlan()
	if request.Msg.GetEventId() == "" || request.Msg.GetPlanVersion() == 0 || alternative == nil || alternative.GetEventId() != request.Msg.GetEventId() || alternative.GetPlanVersion() != request.Msg.GetPlanVersion() {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("matching event, version, and alternative plan required"))
	}
	event, _, err := service.store.Get(ctx, request.Msg.GetEventId())
	if err != nil {
		return nil, storeError(err)
	}
	if event.GetPlanVersion() != request.Msg.GetPlanVersion() || event.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_VALIDATED {
		return nil, storeError(ErrInvalidState)
	}
	frozen, _, err := service.store.LoadPlan(ctx, event.GetEventId(), event.GetPlanVersion())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if frozen == nil || frozen.GetRequestedAt() == nil || frozen.GetReservePolicy() == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("complete frozen input required"))
	}
	canonical := CanonicalFromFrozen(frozen)
	plan, err := safetyPlan(alternative, canonical)
	if err != nil {
		return connect.NewResponse(&gridosv1.ValidateUnsafeAlternativeResponse{
			Violations:          []*gridosv1.SafetyViolation{{Code: string(safety.ContradictoryInput)}},
			OperatorExplanation: err.Error(),
		}), nil
	}
	approval, violations := safety.Validate(plan, canonical)
	response := &gridosv1.ValidateUnsafeAlternativeResponse{Approved: approval.Approved}
	for _, violation := range violations {
		response.Violations = append(response.Violations, &gridosv1.SafetyViolation{Code: string(violation.Code)})
	}
	if len(violations) > 0 {
		response.OperatorExplanation = strings.ToLower(strings.ReplaceAll(string(violations[0].Code), "_", " "))
	}
	return connect.NewResponse(response), nil
}

func CanonicalFromFrozen(request *gridosv1.OptimizationRequest) safety.CanonicalState {
	canonical := safety.CanonicalState{
		Now: request.GetRequestedAt().AsTime(), Boundary: safety.MeterNetExport,
		PolicyVersion: request.GetReservePolicy().GetPolicyVersion(), ExpectedGeneration: int64(request.GetPlanVersion()),
		Devices: make(map[string]safety.DeviceState, len(request.GetDevices())),
	}
	for _, device := range request.GetDevices() {
		energy := device.GetEnergyKwh()
		observedAt := device.GetTelemetryObservedAt().AsTime()
		canonical.Devices[device.GetDeviceId()] = safety.DeviceState{
			EnergyKWh: &energy, UsableCapacityKWh: device.GetUsableEnergyKwh(), HardwareReserveKWh: device.GetHardwareFloorKwh(), PlanReserveKWh: device.GetEffectiveReserveKwh(),
			MaxChargeKW: device.GetMaxChargeKw(), MaxDischargeKW: device.GetMaxDischargeKw(), ChargeEfficiency: device.GetChargeEfficiency(), DischargeEfficiency: device.GetDischargeEfficiency(),
			Available: device.GetAvailabilityProbability() == 1, TelemetryAt: &observedAt, FreshnessLimit: 30 * time.Second, MeterExportLimitKW: device.GetMaxDischargeKw(), InterconnectionLimitKW: device.GetMaxDischargeKw(),
		}
	}
	return canonical
}

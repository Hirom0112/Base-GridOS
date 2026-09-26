package dispatch

import (
	"context"
	"errors"
	"fmt"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	controlapi "github.com/Hirom0112/Base-GridOS/services/control/internal/api"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (activities *Activities) IssueReplacement(ctx context.Context, replacement ReplacementCommand) error {
	event, approved, snapshot, err := activities.loadReplacementInputs(ctx, replacement)
	if err != nil {
		return err
	}
	current := snapshot.Optimization
	now := activities.Now()
	key := fmt.Sprintf("%s-replacement-%d", replacement.EventID, replacement.Generation)
	response, err := activities.Dispatcher.Optimizer.Replace(ctx, &gridosv1.ReplaceRequest{
		Current: current, ApprovedPlan: approved, DroppedDeviceIds: replacement.DroppedDeviceIDs,
		EnvelopeDeviceIds: replacement.EnvelopeDeviceIDs, IdempotencyKey: key,
	})
	if err != nil {
		return err
	}
	plan := response.GetReplacementPlan()
	if err = validateReplacementPlan(replacement, current, plan); err != nil {
		return err
	}
	if len(plan.GetDeviceSchedules()) > 0 {
		snapshot.Canonical.ExpectedGeneration = int64(replacement.Generation)
		if err = activities.Dispatcher.Safety.Validate(plan, snapshot.Canonical); err != nil {
			return err
		}
	}
	commands, err := replacementCommands(replacement, current, plan, key, now)
	if err != nil {
		return err
	}
	if err = storage.NewPostgresEventStore(activities.Pool).StoreReplacement(ctx, replacement.EventID, event.GetPlanVersion(), current, plan, key, now); err != nil {
		return err
	}
	for _, command := range commands {
		if err = storage.InsertCommand(ctx, activities.Pool, command); err != nil {
			return err
		}
	}
	return activities.Dispatcher.Publish(ctx, nil)
}

func (activities *Activities) loadReplacementInputs(ctx context.Context, replacement ReplacementCommand) (*gridosv1.DispatchEvent, *gridosv1.DispatchPlan, controlapi.FrozenSnapshot, error) {
	if err := replacement.validate(); err != nil {
		return nil, nil, controlapi.FrozenSnapshot{}, err
	}
	event, _, err := activities.Events.Get(ctx, replacement.EventID)
	if err != nil {
		return nil, nil, controlapi.FrozenSnapshot{}, err
	}
	if event.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_EXECUTING {
		return nil, nil, controlapi.FrozenSnapshot{}, storage.ErrEventState
	}
	_, approved, err := activities.Events.LoadPlan(ctx, replacement.EventID, event.GetPlanVersion())
	if err != nil {
		return nil, nil, controlapi.FrozenSnapshot{}, err
	}
	approvedIDs := make(map[string]bool, len(approved.GetDeviceSchedules()))
	for _, schedule := range approved.GetDeviceSchedules() {
		approvedIDs[schedule.GetDeviceId()] = true
	}
	for _, id := range replacement.DroppedDeviceIDs {
		if !approvedIDs[id] {
			return nil, nil, controlapi.FrozenSnapshot{}, errors.New("dropped device lacks approved schedule")
		}
	}
	snapshot, err := activities.Dispatcher.Snapshots.Freeze(ctx, event, replacement.Request)
	if err != nil {
		return nil, nil, controlapi.FrozenSnapshot{}, err
	}
	current := snapshot.Optimization
	if current == nil || current.GetPlanVersion() != event.GetPlanVersion()+1 {
		return nil, nil, controlapi.FrozenSnapshot{}, errors.New("fresh next-version replacement snapshot required")
	}
	now := activities.Now()
	for _, interval := range current.GetIntervals() {
		if interval.GetBeginTime() == nil || interval.GetEndTime() == nil || !interval.GetEndTime().AsTime().After(now) {
			return nil, nil, controlapi.FrozenSnapshot{}, errors.New("replacement window has expired")
		}
		if interval.GetBeginTime().AsTime().Before(now) {
			interval.BeginTime = timestamppb.New(now)
		}
	}
	return event, approved, snapshot, nil
}

func (replacement ReplacementCommand) validate() error {
	if replacement.EventID == "" || replacement.Request == nil || replacement.Generation == 0 || len(replacement.DroppedDeviceIDs) == 0 || len(replacement.EnvelopeDeviceIDs) == 0 {
		return errors.New("complete replacement signal required")
	}
	return nil
}

func validateReplacementPlan(replacement ReplacementCommand, current *gridosv1.OptimizationRequest, plan *gridosv1.DispatchPlan) error {
	if plan == nil {
		return errors.New("replacement plan required")
	}
	if plan.GetEventId() != replacement.EventID || plan.GetPlanVersion() != current.GetPlanVersion() {
		return errors.New("replacement plan identity changed")
	}
	envelope := make(map[string]bool, len(replacement.EnvelopeDeviceIDs))
	dropped := make(map[string]bool, len(replacement.DroppedDeviceIDs))
	for _, id := range replacement.EnvelopeDeviceIDs {
		envelope[id] = true
	}
	for _, id := range replacement.DroppedDeviceIDs {
		dropped[id] = true
	}
	for _, schedule := range plan.GetDeviceSchedules() {
		if !envelope[schedule.GetDeviceId()] || dropped[schedule.GetDeviceId()] {
			return errors.New("replacement escaped approved envelope")
		}
	}
	return nil
}

func replacementCommands(replacement ReplacementCommand, current *gridosv1.OptimizationRequest, plan *gridosv1.DispatchPlan, key string, now time.Time) ([]storage.CommandIntent, error) {
	commands := make([]storage.CommandIntent, 0)
	for _, schedule := range plan.GetDeviceSchedules() {
		for index, interval := range schedule.GetIntervals() {
			if interval.GetBeginTime() == nil || interval.GetEndTime() == nil {
				return nil, errors.New("replacement command window required")
			}
			id := fmt.Sprintf("%s-%s-%d", key, schedule.GetDeviceId(), index)
			command := storage.CommandIntent{
				CommandID: id, IdempotencyKey: id, DeviceID: schedule.GetDeviceId(), EventID: replacement.EventID,
				PlanVersion: int64(plan.GetPlanVersion()), Generation: int64(replacement.Generation), SetpointKW: interval.GetSetpointKw(),
				IssuedAt: now, EffectiveAt: interval.GetBeginTime().AsTime(), ExpiresAt: interval.GetEndTime().AsTime(),
				PolicyVersion: current.GetReservePolicy().GetPolicyVersion(), CorrelationID: current.GetCorrelationId(),
			}
			commands = append(commands, command)
		}
	}
	return commands, nil
}

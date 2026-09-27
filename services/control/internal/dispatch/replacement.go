package dispatch

import (
	"context"
	"errors"
	"fmt"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	controlapi "github.com/Hirom0112/Base-GridOS/services/control/internal/api"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/observability"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (activities *Activities) IssueReplacement(ctx context.Context, replacement ReplacementCommand) error {
	ctx, span := activities.startActivity(ctx, replacement.EventID, "IssueReplacement")
	defer span.End()
	if err := replacement.validate(); err != nil {
		return err
	}
	key := fmt.Sprintf("%s-replacement-%d", replacement.EventID, replacement.Generation)
	storedCurrent, storedPlan, err := activities.loadStoredReplacement(ctx, replacement.EventID, key)
	if err == nil {
		return activities.publishReplacement(ctx, replacement, storedCurrent, storedPlan, key)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	event, approved, snapshot, err := activities.loadReplacementInputs(ctx, replacement)
	if err != nil {
		return err
	}
	current := snapshot.Optimization
	now := activities.Now()
	ctx, err = observability.WithTraceIDs(ctx, current.GetCorrelationId(), replacement.EventID)
	if err != nil {
		return err
	}
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
			if err = quantifyRejectedReplacement(plan, approved, replacement.DroppedDeviceIDs, current.GetIntervals()); err != nil {
				return err
			}
		}
	}
	commands, err := replacementCommands(replacement, current, plan, key, now)
	if err != nil {
		return err
	}
	mergedPlan, err := mergeReplacementPlan(approved, plan, replacement.DroppedDeviceIDs)
	if err != nil {
		return err
	}
	if err = storage.NewPostgresEventStore(activities.Pool).StoreReplacement(ctx, replacement.EventID, event.GetPlanVersion(), current, mergedPlan, key, now); err != nil {
		return err
	}
	return activities.publishCommands(ctx, commands)
}

func quantifyRejectedReplacement(plan, approved *gridosv1.DispatchPlan, dropped []string, intervals []*gridosv1.OptimizationInterval) error {
	if len(intervals) == 0 {
		return errors.New("replacement intervals required for shortfall")
	}
	droppedIDs := make(map[string]bool, len(dropped))
	for _, id := range dropped {
		droppedIDs[id] = true
	}
	shortfalls := make([]*gridosv1.ShortfallReport, len(intervals))
	for index, interval := range intervals {
		shortfalls[index] = &gridosv1.ShortfallReport{IntervalBeginTime: interval.GetBeginTime(), IntervalEndTime: interval.GetEndTime(), Reasons: []string{"SAFETY_REJECTED"}}
	}
	for _, schedule := range approved.GetDeviceSchedules() {
		if !droppedIDs[schedule.GetDeviceId()] {
			continue
		}
		if len(schedule.GetIntervals()) != len(intervals) {
			return errors.New("approved replacement interval count changed")
		}
		for index, item := range schedule.GetIntervals() {
			shortfalls[index].RequestedKw += item.GetSetpointKw()
			shortfalls[index].ShortfallKw += item.GetSetpointKw()
		}
	}
	plan.DeviceSchedules = nil
	plan.Shortfalls = shortfalls
	plan.FallbackUsed = true
	plan.FallbackReason = "SAFETY_REJECTED"
	return nil
}

func (activities *Activities) loadStoredReplacement(ctx context.Context, eventID, key string) (*gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, error) {
	var currentJSON, planJSON []byte
	err := activities.Pool.QueryRow(ctx, `SELECT snapshot.inputs, plan.plan FROM audit_journal AS audit
		JOIN plan_versions AS plan ON plan.event_id = audit.resource_id AND plan.version = (audit.new_values->>'plan_version')::bigint
		JOIN input_snapshots AS snapshot ON snapshot.snapshot_id = plan.replacement_snapshot_id
		WHERE audit.resource_id = $1 AND audit.action = 'REPLACEMENT_PLANNED' AND audit.new_values->>'idempotency_key' = $2
		ORDER BY audit.sequence DESC LIMIT 1`, eventID, key).Scan(&currentJSON, &planJSON)
	if err != nil {
		return nil, nil, err
	}
	current := new(gridosv1.OptimizationRequest)
	plan := new(gridosv1.DispatchPlan)
	if err = protojson.Unmarshal(currentJSON, current); err != nil {
		return nil, nil, err
	}
	if err = protojson.Unmarshal(planJSON, plan); err != nil {
		return nil, nil, err
	}
	return current, plan, nil
}

func (activities *Activities) publishReplacement(ctx context.Context, replacement ReplacementCommand, current *gridosv1.OptimizationRequest, plan *gridosv1.DispatchPlan, key string) error {
	_, previous, err := activities.Events.LoadPlan(ctx, replacement.EventID, plan.GetPlanVersion()-1)
	if err != nil {
		return err
	}
	surviving := make(map[string]bool, len(previous.GetDeviceSchedules()))
	for _, schedule := range previous.GetDeviceSchedules() {
		surviving[schedule.GetDeviceId()] = true
	}
	for _, id := range replacement.DroppedDeviceIDs {
		delete(surviving, id)
	}
	newPlan := proto.Clone(plan).(*gridosv1.DispatchPlan)
	newPlan.DeviceSchedules = nil
	for _, schedule := range plan.GetDeviceSchedules() {
		if !surviving[schedule.GetDeviceId()] {
			newPlan.DeviceSchedules = append(newPlan.DeviceSchedules, schedule)
		}
	}
	commands, err := replacementCommands(replacement, current, newPlan, key, activities.Now())
	if err != nil {
		return err
	}
	return activities.publishCommands(ctx, commands)
}

func mergeReplacementPlan(approved, replacement *gridosv1.DispatchPlan, dropped []string) (*gridosv1.DispatchPlan, error) {
	if len(approved.GetShortfalls()) > 0 && len(approved.GetShortfalls()) != len(replacement.GetShortfalls()) {
		return nil, errors.New("replacement shortfall interval count changed")
	}
	merged := proto.Clone(replacement).(*gridosv1.DispatchPlan)
	droppedIDs := make(map[string]bool, len(dropped))
	for _, id := range dropped {
		droppedIDs[id] = true
	}
	merged.DeviceSchedules = nil
	for _, schedule := range approved.GetDeviceSchedules() {
		if !droppedIDs[schedule.GetDeviceId()] {
			merged.DeviceSchedules = append(merged.DeviceSchedules, proto.Clone(schedule).(*gridosv1.DeviceSchedule))
		}
	}
	merged.DeviceSchedules = append(merged.DeviceSchedules, replacement.GetDeviceSchedules()...)
	if len(approved.GetShortfalls()) == len(merged.GetShortfalls()) {
		for index, previous := range approved.GetShortfalls() {
			merged.Shortfalls[index].RequestedKw = previous.GetRequestedKw()
			merged.Shortfalls[index].ShortfallKw += previous.GetShortfallKw()
			merged.Shortfalls[index].FeasibleKw = max(0, merged.Shortfalls[index].RequestedKw-merged.Shortfalls[index].ShortfallKw)
			merged.Shortfalls[index].Reasons = append(merged.Shortfalls[index].Reasons, previous.GetReasons()...)
		}
	}
	return merged, nil
}

func (activities *Activities) publishCommands(ctx context.Context, commands []storage.CommandIntent) error {
	for _, command := range commands {
		var planVersion, generation int64
		var setpoint float64
		err := activities.Pool.QueryRow(ctx, `SELECT plan_version, generation, setpoint_kw FROM command_intents WHERE command_id = $1`, command.CommandID).Scan(&planVersion, &generation, &setpoint)
		if err == nil {
			if planVersion != command.PlanVersion || generation != command.Generation || setpoint != command.SetpointKW {
				return errors.New("replacement command changed on retry")
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
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

package reconciliation

import (
	"context"
	"errors"
	"slices"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type RecoveryDrop struct {
	DeviceID string
	Reason   string
}

type RecoveryDecision struct {
	Dropped           []RecoveryDrop
	EnvelopeDeviceIDs []string
}

type recoveryCommand struct {
	state      string
	setpointKW float64
}

type recoveryObservation struct {
	observedAt time.Time
	valueState string
}

func (activities *Activities) DetectRecovery(ctx context.Context, input Input) (RecoveryDecision, error) {
	if input.EventID == "" || activities.MaxGap <= 0 {
		return RecoveryDecision{}, errors.New("event ID and telemetry max gap required")
	}
	now := activities.Now()
	var state string
	var begin, end time.Time
	var planJSON []byte
	var envelope []string
	err := activities.Pool.QueryRow(ctx, `SELECT event.state, request.begin_time, request.end_time, plan.plan, eligibility.eligible_device_ids
		FROM dispatch_events AS event JOIN dispatch_requests AS request USING (request_id)
		JOIN plan_versions AS plan ON plan.event_id = event.event_id AND plan.version = event.plan_version
		JOIN eligibility_snapshots AS eligibility ON eligibility.snapshot_id = plan.eligibility_snapshot_id
		WHERE event.event_id = $1`, input.EventID).Scan(&state, &begin, &end, &planJSON, &envelope)
	if err != nil {
		return RecoveryDecision{}, err
	}
	if state != "EXECUTING" || now.Before(begin) || !now.Before(end) {
		return RecoveryDecision{}, nil
	}
	plan := new(gridosv1.DispatchPlan)
	if err := protojson.Unmarshal(planJSON, plan); err != nil {
		return RecoveryDecision{}, err
	}
	ids := make([]string, 0, len(plan.GetDeviceSchedules()))
	for _, schedule := range plan.GetDeviceSchedules() {
		ids = append(ids, schedule.GetDeviceId())
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	slices.Sort(envelope)
	decision := RecoveryDecision{EnvelopeDeviceIDs: envelope}
	if len(ids) == 0 {
		return decision, nil
	}
	commands, err := activities.recoveryCommands(ctx, input.EventID, ids, now)
	if err != nil {
		return RecoveryDecision{}, err
	}
	observations, err := activities.recoveryObservations(ctx, ids, now)
	if err != nil {
		return RecoveryDecision{}, err
	}
	for _, id := range ids {
		command, found := commands[id]
		if !found || command.setpointKW == 0 {
			continue
		}
		reason := recoveryReason(command, observations[id], begin, now, activities.MaxGap)
		if reason != "" {
			decision.Dropped = append(decision.Dropped, RecoveryDrop{DeviceID: id, Reason: reason})
		}
	}
	return decision, nil
}

func recoveryReason(command recoveryCommand, observation recoveryObservation, begin, now time.Time, maxGap time.Duration) string {
	switch {
	case command.state == "REJECTED":
		return "REJECTED"
	case command.state == "UNCERTAIN":
		return "UNCERTAIN"
	case recoveryMissing(observation, begin, now, maxGap):
		return "MISSING"
	default:
		return ""
	}
}

func (activities *Activities) recoveryCommands(ctx context.Context, eventID string, ids []string, now time.Time) (map[string]recoveryCommand, error) {
	rows, err := activities.Pool.Query(ctx, `SELECT DISTINCT ON (intent.device_id) intent.device_id, latest.state, intent.setpoint_kw
		FROM command_intents AS intent JOIN LATERAL (
			SELECT state FROM command_states WHERE command_id = intent.command_id ORDER BY recorded_at DESC LIMIT 1
		) AS latest ON true
		WHERE intent.event_id = $1 AND intent.device_id = ANY($2) AND intent.effective_at <= $3 AND intent.expires_at > $3
		ORDER BY intent.device_id, intent.generation DESC, intent.issued_at DESC, intent.command_id DESC`, eventID, ids, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	commands := make(map[string]recoveryCommand, len(ids))
	for rows.Next() {
		var id string
		var command recoveryCommand
		if err := rows.Scan(&id, &command.state, &command.setpointKW); err != nil {
			return nil, err
		}
		commands[id] = command
	}
	return commands, rows.Err()
}

func (activities *Activities) recoveryObservations(ctx context.Context, ids []string, now time.Time) (map[string]recoveryObservation, error) {
	rows, err := activities.Pool.Query(ctx, `SELECT DISTINCT ON (device_id) device_id, observed_at, COALESCE(payload->>'valueState', '')
		FROM telemetry_observations WHERE device_id = ANY($1) AND observed_at <= $2
		ORDER BY device_id, observed_at DESC, sequence DESC`, ids, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	observations := make(map[string]recoveryObservation, len(ids))
	for rows.Next() {
		var id string
		var observation recoveryObservation
		if err := rows.Scan(&id, &observation.observedAt, &observation.valueState); err != nil {
			return nil, err
		}
		observations[id] = observation
	}
	return observations, rows.Err()
}

func recoveryMissing(observation recoveryObservation, begin, now time.Time, maxGap time.Duration) bool {
	if observation.valueState == "VALUE_STATE_MISSING" && !observation.observedAt.Before(begin) {
		return true
	}
	if observation.observedAt.IsZero() {
		return now.Sub(begin) >= maxGap
	}
	return now.Sub(observation.observedAt) > maxGap
}

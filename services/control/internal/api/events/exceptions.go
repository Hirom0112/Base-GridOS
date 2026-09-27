package events

import (
	"context"
	"fmt"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (source *PostgresSource) TimelineExceptions(ctx context.Context, eventID string) ([]*gridosv1.EventException, error) {
	rows, err := source.pool.Query(ctx, `WITH event_window AS (
		SELECT request.begin_time, request.end_time FROM dispatch_requests AS request
		JOIN dispatch_events AS event USING (request_id) WHERE event.event_id = $1
	), missing AS (
		SELECT DISTINCT ON (observation.device_id) 'MISSING_TELEMETRY' AS kind,
			observation.observed_at AS occurred_at, observation.device_id, ''::text AS command_id,
			observation.observation_id AS evidence_id,
			count(*) OVER (PARTITION BY observation.device_id)::text || ' missing observations in window' AS detail
		FROM telemetry_observations AS observation CROSS JOIN event_window AS bounds
		WHERE observation.observed_at >= bounds.begin_time AND observation.observed_at < bounds.end_time
		AND observation.payload->>'valueState' = 'VALUE_STATE_MISSING'
		AND EXISTS (SELECT 1 FROM command_intents AS intent WHERE intent.event_id = $1 AND intent.device_id = observation.device_id)
		ORDER BY observation.device_id, observation.observed_at, observation.sequence
	), uncertain AS (
		SELECT 'UNCERTAIN_COMMAND' AS kind, state.recorded_at AS occurred_at, intent.device_id,
			intent.command_id, intent.command_id AS evidence_id, 'acknowledgement deadline elapsed' AS detail
		FROM command_states AS state JOIN command_intents AS intent USING (command_id)
		WHERE intent.event_id = $1 AND state.state = 'UNCERTAIN'
	), rejected AS (
		SELECT 'REJECTED_COMMAND' AS kind, state.recorded_at AS occurred_at, intent.device_id,
			intent.command_id, intent.command_id AS evidence_id, 'gateway rejected command' AS detail
		FROM command_states AS state JOIN command_intents AS intent USING (command_id)
		WHERE intent.event_id = $1 AND state.state = 'REJECTED'
	), late AS (
		SELECT 'LATE_ACCEPTANCE' AS kind, ack.received_at AS occurred_at, intent.device_id,
			intent.command_id, ack.acknowledgement_id AS evidence_id, 'accepted after uncertain transition' AS detail
		FROM command_acknowledgements AS ack JOIN command_intents AS intent USING (command_id)
		WHERE intent.event_id = $1 AND ack.receipt_status = 'ACCEPTED'
		AND EXISTS (SELECT 1 FROM command_states AS state WHERE state.command_id = ack.command_id
			AND state.state = 'UNCERTAIN' AND state.recorded_at < ack.received_at)
	), retry AS (
		SELECT 'COMMAND_RETRY' AS kind, outbox.published_at AS occurred_at, intent.device_id,
			intent.command_id, intent.command_id AS evidence_id, outbox.attempts::text || ' publish attempts' AS detail
		FROM command_outbox AS outbox JOIN command_intents AS intent USING (command_id)
		WHERE intent.event_id = $1 AND outbox.attempts > 1 AND outbox.published_at IS NOT NULL
	), recovery AS (
		SELECT audit.action AS kind, audit.occurred_at, ''::text AS device_id, ''::text AS command_id,
			audit.sequence::text AS evidence_id, audit.new_values::text AS detail
		FROM audit_journal AS audit WHERE audit.resource_id = $1
		AND audit.action IN ('REPLACEMENT_PLANNED', 'REPLACEMENT_SAFETY_REJECTED', 'EMERGENCY_STOP_REQUESTED')
	), removed AS (
		SELECT 'STALE_CAPACITY_REMOVED' AS kind, audit.occurred_at, prior.schedule->>'deviceId' AS device_id,
			''::text AS command_id, audit.sequence::text AS evidence_id, 'removed from replacement plan' AS detail
		FROM audit_journal AS audit
		JOIN dispatch_events AS event ON event.event_id = audit.resource_id
		JOIN dispatch_requests AS request USING (request_id)
		JOIN plan_versions AS current ON current.event_id = audit.resource_id
			AND current.version = (audit.new_values->>'plan_version')::bigint
		JOIN plan_versions AS previous ON previous.event_id = audit.resource_id AND previous.version = current.version - 1
		CROSS JOIN LATERAL jsonb_array_elements(COALESCE(previous.plan->'deviceSchedules', '[]'::jsonb)) AS prior(schedule)
		WHERE audit.resource_id = $1 AND audit.action = 'REPLACEMENT_PLANNED'
		AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(current.plan->'deviceSchedules', '[]'::jsonb)) AS next(schedule)
			WHERE next.schedule->>'deviceId' = prior.schedule->>'deviceId')
		AND EXISTS (SELECT 1 FROM telemetry_observations AS observation
			WHERE observation.device_id = prior.schedule->>'deviceId'
			AND observation.payload->>'valueState' = 'VALUE_STATE_MISSING'
			AND observation.observed_at >= request.begin_time AND observation.observed_at < request.end_time
			AND observation.observed_at <= audit.occurred_at)
	), rebalanced AS (
		SELECT 'REBALANCED_COMMAND' AS kind, intent.issued_at AS occurred_at, intent.device_id,
			intent.command_id, intent.command_id AS evidence_id, 'replacement command issued' AS detail
		FROM command_intents AS intent WHERE intent.event_id = $1 AND intent.setpoint_kw <> 0
		AND EXISTS (SELECT 1 FROM audit_journal AS audit WHERE audit.resource_id = intent.event_id
			AND audit.action = 'REPLACEMENT_PLANNED' AND (audit.new_values->>'plan_version')::bigint = intent.plan_version)
	)
	SELECT kind, occurred_at, device_id, command_id, evidence_id, detail FROM missing
	UNION ALL SELECT kind, occurred_at, device_id, command_id, evidence_id, detail FROM uncertain
	UNION ALL SELECT kind, occurred_at, device_id, command_id, evidence_id, detail FROM rejected
	UNION ALL SELECT kind, occurred_at, device_id, command_id, evidence_id, detail FROM late
	UNION ALL SELECT kind, occurred_at, device_id, command_id, evidence_id, detail FROM retry
	UNION ALL SELECT kind, occurred_at, device_id, command_id, evidence_id, detail FROM recovery
	UNION ALL SELECT kind, occurred_at, device_id, command_id, evidence_id, detail FROM removed
	UNION ALL SELECT kind, occurred_at, device_id, command_id, evidence_id, detail FROM rebalanced
	ORDER BY occurred_at, kind, evidence_id`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	exceptions := make([]*gridosv1.EventException, 0)
	for rows.Next() {
		var kind, deviceID, commandID, evidenceID, detail string
		var occurredAt time.Time
		if err = rows.Scan(&kind, &occurredAt, &deviceID, &commandID, &evidenceID, &detail); err != nil {
			return nil, err
		}
		value, ok := gridosv1.EventExceptionKind_value["EVENT_EXCEPTION_KIND_"+kind]
		if !ok {
			return nil, fmt.Errorf("unknown event exception kind %q", kind)
		}
		exceptions = append(exceptions, &gridosv1.EventException{Kind: gridosv1.EventExceptionKind(value), OccurredAt: timestamppb.New(occurredAt), EventId: eventID, DeviceId: deviceID, CommandId: commandID, EvidenceId: evidenceID, Detail: detail})
	}
	return exceptions, rows.Err()
}

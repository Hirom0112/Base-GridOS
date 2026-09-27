package events

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) ListEventCommands(ctx context.Context, request *connect.Request[gridosv1.ListEventCommandsRequest]) (*connect.Response[gridosv1.ListEventCommandsResponse], error) {
	if err := authorize(request.Header().Get("X-GridOS-Role"), "operator", "approver", "analyst", "service"); err != nil {
		return nil, err
	}
	if request.Msg.GetEventId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event identifier is required"))
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := service.source.Commands(ctx, request.Msg.GetEventId())
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(response), nil
}

func (source *PostgresSource) Commands(ctx context.Context, eventID string) (*gridosv1.ListEventCommandsResponse, error) {
	var found string
	if err := source.pool.QueryRow(ctx, `SELECT event_id FROM dispatch_events WHERE event_id = $1`, eventID).Scan(&found); err != nil {
		return nil, err
	}
	response := &gridosv1.ListEventCommandsResponse{}
	rows, err := source.pool.Query(ctx, `SELECT intent.command_id, intent.idempotency_key, intent.device_id, intent.event_id,
		intent.plan_version, intent.generation, intent.setpoint_kw, intent.issued_at, intent.effective_at,
		intent.expires_at, intent.policy_version, intent.correlation_id, latest.state, latest.recorded_at,
		receipt.acknowledgement_id, receipt.idempotency_key, receipt.receipt_status, receipt.received_at,
		receipt.gateway_id, receipt.rejection_reason
		FROM command_intents AS intent
		LEFT JOIN LATERAL (SELECT state, recorded_at FROM command_states WHERE command_id = intent.command_id
			ORDER BY recorded_at DESC LIMIT 1) AS latest ON true
		LEFT JOIN LATERAL (SELECT acknowledgement_id, idempotency_key, receipt_status, received_at,
			gateway_id, rejection_reason FROM command_acknowledgements WHERE command_id = intent.command_id
			ORDER BY received_at DESC, acknowledgement_id DESC LIMIT 1) AS receipt ON true
		WHERE intent.event_id = $1 ORDER BY intent.issued_at, intent.command_id`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		command, scanErr := scanEventCommand(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		response.Commands = append(response.Commands, command)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	intervals, err := source.pool.Query(ctx, `SELECT interval_begin_time, interval_end_time, requested_kw, commanded_kw,
		delivered_kw, tracking_error_kw, confidence, baseline_method, measurement_boundary
		FROM verification_summaries WHERE event_id = $1 ORDER BY interval_begin_time, verification_id`, eventID)
	if err != nil {
		return nil, err
	}
	defer intervals.Close()
	for intervals.Next() {
		var begin, end time.Time
		verified := &gridosv1.EventIntervalVerification{ValueKind: "MEASURED"}
		if err := intervals.Scan(&begin, &end, &verified.RequestedKw, &verified.CommandedKw, &verified.DeliveredKw,
			&verified.TrackingErrorKw, &verified.Confidence, &verified.BaselineMethod, &verified.MeasurementBoundary); err != nil {
			return nil, err
		}
		verified.BeginTime, verified.EndTime = timestamppb.New(begin), timestamppb.New(end)
		response.VerificationIntervals = append(response.VerificationIntervals, verified)
	}
	return response, intervals.Err()
}

func scanEventCommand(rows pgx.Rows) (*gridosv1.EventCommand, error) {
	intent := &gridosv1.CommandIntent{}
	var version, generation int64
	var issued, effective, expires time.Time
	var state, receiptID, receiptKey, receiptStatus, gatewayID, rejection pgtype.Text
	var stateAt, receivedAt pgtype.Timestamptz
	err := rows.Scan(&intent.CommandId, &intent.IdempotencyKey, &intent.DeviceId, &intent.EventId,
		&version, &generation, &intent.SetpointKw, &issued, &effective, &expires,
		&intent.PolicyVersion, &intent.CorrelationId, &state, &stateAt,
		&receiptID, &receiptKey, &receiptStatus, &receivedAt, &gatewayID, &rejection)
	if err != nil {
		return nil, err
	}
	if version <= 0 || generation < 0 || !state.Valid || !stateAt.Valid {
		return nil, errors.New("stored command is missing a valid generation or durable state")
	}
	intent.PlanVersion, intent.Generation = uint64(version), uint64(generation)
	intent.IssuedAt, intent.EffectiveAt, intent.ExpiresAt = timestamppb.New(issued), timestamppb.New(effective), timestamppb.New(expires)
	lifecycle, found := commandLifecycle[state.String]
	if !found {
		return nil, errors.New("stored command state is invalid")
	}
	command := &gridosv1.EventCommand{Intent: intent, LifecycleState: lifecycle, StateRecordedAt: timestamppb.New(stateAt.Time)}
	if !receiptID.Valid {
		return command, nil
	}
	if !receiptKey.Valid || !receiptStatus.Valid || !receivedAt.Valid || !gatewayID.Valid || !rejection.Valid {
		return nil, errors.New("stored command receipt is incomplete")
	}
	var status gridosv1.CommandReceiptStatus
	switch receiptStatus.String {
	case "ACCEPTED":
		status = gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_ACCEPTED
	case "REJECTED":
		status = gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_REJECTED
	default:
		return nil, errors.New("stored command receipt status is invalid")
	}
	command.Receipt = &gridosv1.CommandAcknowledgement{AcknowledgementId: receiptID.String, CommandId: intent.CommandId,
		IdempotencyKey: receiptKey.String, ReceiptStatus: status, ReceivedAt: timestamppb.New(receivedAt.Time),
		GatewayId: gatewayID.String, RejectionReason: rejection.String}
	return command, nil
}

var commandLifecycle = map[string]gridosv1.CommandLifecycleState{
	"PERSISTED":    gridosv1.CommandLifecycleState_COMMAND_LIFECYCLE_STATE_PERSISTED,
	"SENT":         gridosv1.CommandLifecycleState_COMMAND_LIFECYCLE_STATE_SENT,
	"ACKNOWLEDGED": gridosv1.CommandLifecycleState_COMMAND_LIFECYCLE_STATE_ACKNOWLEDGED,
	"UNCERTAIN":    gridosv1.CommandLifecycleState_COMMAND_LIFECYCLE_STATE_UNCERTAIN,
	"EXECUTING":    gridosv1.CommandLifecycleState_COMMAND_LIFECYCLE_STATE_EXECUTING,
	"COMPLETED":    gridosv1.CommandLifecycleState_COMMAND_LIFECYCLE_STATE_COMPLETED,
	"CANCELLED":    gridosv1.CommandLifecycleState_COMMAND_LIFECYCLE_STATE_CANCELLED,
	"EXPIRED":      gridosv1.CommandLifecycleState_COMMAND_LIFECYCLE_STATE_EXPIRED,
	"REJECTED":     gridosv1.CommandLifecycleState_COMMAND_LIFECYCLE_STATE_REJECTED,
}

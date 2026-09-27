package events

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestListEventCommandsFromDurableRows(t *testing.T) {
	pool := exceptionDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `INSERT INTO dispatch_requests (request_id, event_type, begin_time, end_time, target_kw, measurement_boundary, load_zones, correlation_id)
		VALUES ('request-commands', 'GRID_SERVICE', $1, $1::timestamptz + interval '1 hour', 10, 'METER_NET_EXPORT', ARRAY['LZ_AEN'], 'commands');
		INSERT INTO dispatch_events (event_id, request_id, state, plan_version, correlation_id)
		VALUES ('event-commands', 'request-commands', 'REPORTED', 1, 'commands');
		INSERT INTO input_snapshots (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
		VALUES ('input-commands', 'event-commands', $1, '{}', '{}', 'commands');
		INSERT INTO eligibility_snapshots (snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
		VALUES ('eligibility-commands', 'event-commands', $1, ARRAY['device-1', 'device-2'], '[]', 'policy-1', 'commands');
		INSERT INTO plan_versions (event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
		VALUES ('event-commands', 1, 'input-commands', 'eligibility-commands', '{}', 'solver-1', 'model-1', 'commands');
		INSERT INTO command_intents (command_id, idempotency_key, device_id, event_id, plan_version, generation, setpoint_kw, issued_at, effective_at, expires_at, policy_version, correlation_id)
		VALUES ('command-ack', 'key-ack', 'device-1', 'event-commands', 1, 4, 10, $1, $1, $1::timestamptz + interval '1 hour', 'policy-1', 'commands'),
		('command-expired', 'key-expired', 'device-2', 'event-commands', 1, 5, 0, $1::timestamptz + interval '1 minute', $1::timestamptz + interval '1 minute', $1::timestamptz + interval '2 minutes', 'policy-1', 'commands');
		INSERT INTO command_states (command_id, state, recorded_at, correlation_id)
		VALUES ('command-ack', 'PERSISTED', $1, 'commands'), ('command-ack', 'ACKNOWLEDGED', $1::timestamptz + interval '1 minute', 'commands'),
		('command-expired', 'PERSISTED', $1::timestamptz + interval '1 minute', 'commands'), ('command-expired', 'EXPIRED', $1::timestamptz + interval '3 minutes', 'commands');
		INSERT INTO command_acknowledgements (acknowledgement_id, command_id, idempotency_key, receipt_status, received_at, gateway_id, correlation_id)
		VALUES ('receipt-ack', 'command-ack', 'receipt-key', 'ACCEPTED', $1::timestamptz + interval '1 minute', 'gateway-1', 'commands');
		INSERT INTO verification_summaries (verification_id, event_id, interval_begin_time, interval_end_time, requested_kw, commanded_kw, delivered_kw, tracking_error_kw, confidence, baseline_method, measurement_boundary, correlation_id)
		VALUES ('verification-commands', 'event-commands', $1, $1::timestamptz + interval '5 minutes', 10, 10, 9, 1, 0.8, 'MEASURED_AT_BOUNDARY', 'METER_NET_EXPORT', 'commands')`, pgx.QueryExecModeSimpleProtocol, begin)
	require.NoError(t, err)
	service := NewService(NewPostgresSource(pool, nil, nil, func() time.Time { return begin }, time.Minute), time.Second)
	request := connect.NewRequest(&gridosv1.ListEventCommandsRequest{EventId: "event-commands"})
	request.Header().Set("X-GridOS-Role", "analyst")
	response, err := service.ListEventCommands(ctx, request)
	require.NoError(t, err)
	require.Len(t, response.Msg.GetCommands(), 2)
	acknowledged := response.Msg.GetCommands()[0]
	require.Equal(t, "command-ack", acknowledged.GetIntent().GetCommandId())
	require.Equal(t, uint64(4), acknowledged.GetIntent().GetGeneration())
	require.Equal(t, "ACKNOWLEDGED", acknowledged.GetState())
	require.Equal(t, begin.Add(time.Minute), acknowledged.GetStateRecordedAt().AsTime())
	require.Equal(t, gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_ACCEPTED, acknowledged.GetReceipt().GetReceiptStatus())
	require.Equal(t, "receipt-ack", acknowledged.GetReceipt().GetAcknowledgementId())
	require.Equal(t, begin.Add(time.Hour), acknowledged.GetIntent().GetExpiresAt().AsTime())
	expired := response.Msg.GetCommands()[1]
	require.Equal(t, "EXPIRED", expired.GetState())
	require.Nil(t, expired.GetReceipt())
	require.Equal(t, begin.Add(2*time.Minute), expired.GetIntent().GetExpiresAt().AsTime())
	require.Len(t, response.Msg.GetVerificationIntervals(), 1)
	interval := response.Msg.GetVerificationIntervals()[0]
	require.Equal(t, "MEASURED", interval.GetValueKind())
	require.Equal(t, 10.0, interval.GetCommandedKw())
	require.Equal(t, 9.0, interval.GetDeliveredKw())
	require.Equal(t, 1.0, interval.GetTrackingErrorKw())
	require.Equal(t, 0.8, interval.GetConfidence())
	require.Equal(t, begin.Add(5*time.Minute), interval.GetEndTime().AsTime())
	require.Equal(t, "METER_NET_EXPORT", interval.GetMeasurementBoundary())
	request.Msg.EventId = "missing-event"
	_, err = service.ListEventCommands(ctx, request)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestListEventCommandsAuthorization(t *testing.T) {
	service := NewService(&changingSource{}, time.Second)
	request := connect.NewRequest(&gridosv1.ListEventCommandsRequest{EventId: "event-1"})
	_, err := service.ListEventCommands(context.Background(), request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	request.Header().Set("X-GridOS-Role", "member")
	_, err = service.ListEventCommands(context.Background(), request)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	request.Header().Set("X-GridOS-Role", "operator")
	request.Msg.EventId = ""
	_, err = service.ListEventCommands(context.Background(), request)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	request.Msg.EventId = "event-1"
	_, err = service.ListEventCommands(context.Background(), request)
	require.NoError(t, err)
}

func TestEventCommandLifecycleEnum(t *testing.T) {
	fields := (&gridosv1.EventCommand{}).ProtoReflect().Descriptor().Fields()
	if fields.ByName("state") == nil {
		t.Fatal("event command contract is unavailable")
	}
	field := fields.ByName("lifecycle_state")
	if field == nil || field.Kind().String() != "enum" {
		t.Fatal("event command lifecycle must be a closed enum")
	}
}

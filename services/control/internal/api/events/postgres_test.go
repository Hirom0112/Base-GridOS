package events

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTimelineSurfacesMalformedAuditValues(t *testing.T) {
	pool := exceptionDatabase(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `INSERT INTO audit_journal (occurred_at, actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1, 'operator', 'EVENT_STATE_TRANSITIONED', 'dispatch_event', 'event-wellformed', '{"state":"sent","reason":"published"}', 'audit'),
		($1, 'operator', 'EVENT_STATE_TRANSITIONED', 'dispatch_event', 'event-malformed', '{"state":7}', 'audit')`, pgx.QueryExecModeSimpleProtocol, at)
	if err != nil {
		t.Fatal(err)
	}
	source := NewPostgresSource(pool, nil, nil, func() time.Time { return at }, time.Minute)
	entries, err := source.Timeline(ctx, "event-wellformed")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_SENT || entries[0].GetReason() != "published" {
		t.Fatalf("well-formed timeline = %v", entries)
	}
	entries, err = source.Timeline(ctx, "event-malformed")
	if err == nil {
		t.Fatalf("malformed audit values decoded silently as %v", entries)
	}
}

type recordedStops struct{}

func (recordedStops) RequestEmergencyStop(context.Context, string, string) error { return nil }

func TestEmergencyStopAuditsTheServerClockNotTheClientClock(t *testing.T) {
	pool := exceptionDatabase(t)
	ctx := context.Background()
	serverNow := time.Date(2026, 9, 27, 14, 58, 1, 371000000, time.UTC)
	clientEarlier := serverNow.Add(-16 * time.Millisecond)
	_, err := pool.Exec(ctx, `INSERT INTO dispatch_requests (request_id, event_type, begin_time, end_time, target_kw, measurement_boundary, load_zones, correlation_id)
		VALUES ('request-stop', 'GRID_SERVICE', $1, $1::timestamptz + interval '1 hour', 100, 'METER_NET_EXPORT', ARRAY['LZ_AEN'], 'stop');
		INSERT INTO dispatch_events (event_id, request_id, state, plan_version, correlation_id)
		VALUES ('event-stop', 'request-stop', 'SENT', 1, 'stop')`, pgx.QueryExecModeSimpleProtocol, serverNow)
	if err != nil {
		t.Fatal(err)
	}
	source := NewPostgresSource(pool, recordedStops{}, nil, func() time.Time { return serverNow }, time.Minute)
	_, err = source.RequestStop(ctx, &gridosv1.EmergencyStopRequest{EventId: "event-stop", IdempotencyKey: "stop-1", RequestedBy: "operator-1", Reason: "drill", RequestedAt: timestamppb.New(clientEarlier), CorrelationId: "stop"})
	if err != nil {
		t.Fatal(err)
	}
	var occurredAt time.Time
	if err = pool.QueryRow(ctx, `SELECT occurred_at FROM audit_journal WHERE action = 'EMERGENCY_STOP_REQUESTED' AND resource_id = 'event-stop'`).Scan(&occurredAt); err != nil {
		t.Fatal(err)
	}
	if !occurredAt.Equal(serverNow) {
		t.Fatalf("EMERGENCY_STOP_REQUESTED occurred_at = %s, want server clock %s", occurredAt, serverNow)
	}
}

package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type eventStoreContract interface {
	Create(context.Context, *gridosv1.EventRequest, string, time.Time) (*gridosv1.DispatchEvent, error)
	Get(context.Context, string) (*gridosv1.DispatchEvent, map[gridosv1.ExclusionReason]uint64, error)
	Approve(context.Context, *gridosv1.ApproveEventRequest) (*gridosv1.DispatchEvent, error)
	Launch(context.Context, *gridosv1.LaunchEventRequest) (*gridosv1.DispatchEvent, error)
}

func TestPostgresEventStorePersistsApprovalLaunchAndExclusions(t *testing.T) {
	pool := testDatabase(t)
	store := NewPostgresEventStore(pool)
	var _ eventStoreContract = store
	now := time.Now().UTC()
	event, err := store.Create(context.Background(), &gridosv1.EventRequest{
		RequestId: "stored-event", EventType: "GRID_SERVICE",
		BeginTime: timestamppb.New(now.Add(time.Hour)), EndTime: timestamppb.New(now.Add(2 * time.Hour)),
		TargetKw: 20000, MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT,
		LoadZones: []string{"LZ_AEN"}, CorrelationId: "correlation-store",
	}, "create-store", now)
	if err != nil {
		t.Fatal(err)
	}
	if event.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_REQUESTED {
		t.Fatalf("created state = %s", event.GetState())
	}
	seedStoredPlan(t, pool, event.GetEventId(), now)
	approved, err := store.Approve(context.Background(), &gridosv1.ApproveEventRequest{
		EventId: event.GetEventId(), PlanVersion: 3, IdempotencyKey: "approve-store",
		ApprovedBy: "approver-1", ApprovedAt: timestamppb.New(now.Add(time.Minute)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if approved.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_APPROVED {
		t.Fatalf("approved state = %s", approved.GetState())
	}
	launched, err := store.Launch(context.Background(), &gridosv1.LaunchEventRequest{
		EventId: event.GetEventId(), PlanVersion: 3, IdempotencyKey: "launch-store",
		RequestedBy: "approver-1", RequestedAt: timestamppb.New(now.Add(2 * time.Minute)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if launched.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_COMMANDS_PERSISTED {
		t.Fatalf("launched state = %s", launched.GetState())
	}
	restarted := NewPostgresEventStore(pool)
	loaded, exclusions, err := restarted.Get(context.Background(), event.GetEventId())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.GetLaunch().GetRequestedBy() != "approver-1" || loaded.GetLaunch().GetPlanVersion() != 3 {
		t.Fatalf("stored launch = %#v", loaded.GetLaunch())
	}
	if exclusions[gridosv1.ExclusionReason_EXCLUSION_REASON_RESERVE] != 2 || exclusions[gridosv1.ExclusionReason_EXCLUSION_REASON_STALE_TELEMETRY] != 1 {
		t.Fatalf("grouped exclusions = %v", exclusions)
	}
	var approvals, actions int
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM operator_approvals WHERE event_id = $1", event.GetEventId()).Scan(&approvals); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM audit_journal WHERE resource_id = $1 AND action IN ('EVENT_APPROVED', 'EVENT_LAUNCHED')", event.GetEventId()).Scan(&actions); err != nil {
		t.Fatal(err)
	}
	if approvals != 1 || actions != 2 {
		t.Fatalf("approval/audit rows = %d/%d", approvals, actions)
	}
	var previousState, nextState string
	if err = pool.QueryRow(context.Background(), `SELECT previous_values->>'state', new_values->>'state'
		FROM audit_journal WHERE resource_id = $1 AND action = 'EVENT_STATE_TRANSITIONED'
		ORDER BY sequence DESC LIMIT 1`, event.GetEventId()).Scan(&previousState, &nextState); err != nil {
		t.Fatal(err)
	}
	if previousState != "APPROVED" || nextState != "COMMANDS_PERSISTED" {
		t.Fatalf("launch transition = %s -> %s", previousState, nextState)
	}
}

func TestPostgresEventStoreRejectsWrongPlanVersion(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-plan-version")
	_, err := pool.Exec(context.Background(), "UPDATE dispatch_events SET state = 'VALIDATED', plan_version = 1 WHERE event_id = 'event-plan-version'")
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewPostgresEventStore(pool).Approve(context.Background(), &gridosv1.ApproveEventRequest{
		EventId: "event-plan-version", PlanVersion: 2, IdempotencyKey: "wrong-plan",
		ApprovedBy: "approver-1", ApprovedAt: timestamppb.Now(),
	})
	if !errors.Is(err, ErrEventPlanVersion) {
		t.Fatalf("error = %v, want ErrEventPlanVersion", err)
	}
}

func seedStoredPlan(t *testing.T, pool *pgxpool.Pool, eventID string, now time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO input_snapshots
        (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
        VALUES ('stored-input', $1, $2, '{}', '{}', 'correlation-store')`, eventID, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO eligibility_snapshots
        (snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
        VALUES ('stored-eligibility', $1, $2, ARRAY['device-1'],
        '[{"device_id":"device-2","reason":"EXCLUSION_REASON_RESERVE"},{"device_id":"device-3","reason":"EXCLUSION_REASON_RESERVE"},{"device_id":"device-4","reason":"EXCLUSION_REASON_STALE_TELEMETRY"}]',
        'policy-1', 'correlation-store')`, eventID, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO plan_versions
        (event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
		VALUES ($1, 3, 'stored-input', 'stored-eligibility', '{}', 'solver-1', 'model-1', 'correlation-store')`, eventID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, "UPDATE dispatch_events SET state = 'VALIDATED', plan_version = 3 WHERE event_id = $1", eventID)
	if err != nil {
		t.Fatal(err)
	}
}

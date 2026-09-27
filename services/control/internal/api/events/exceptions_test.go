package events

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestExceptionsFromDurableEvidence(t *testing.T) {
	pool := exceptionDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `INSERT INTO dispatch_requests (request_id, event_type, begin_time, end_time, target_kw, measurement_boundary, load_zones, correlation_id)
		VALUES ('request-exception', 'GRID_SERVICE', $1, $1::timestamptz + interval '1 hour', 100, 'METER_NET_EXPORT', ARRAY['LZ_AEN'], 'exception');
		INSERT INTO dispatch_events (event_id, request_id, state, plan_version, correlation_id)
		VALUES ('event-exception', 'request-exception', 'SENT', 1, 'exception');
		INSERT INTO input_snapshots (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
		VALUES ('input-exception', 'event-exception', $1, '{}', '{}', 'exception');
		INSERT INTO eligibility_snapshots (snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
		VALUES ('eligibility-exception', 'event-exception', $1, ARRAY['device-1'], '[]', 'policy-1', 'exception');
		INSERT INTO plan_versions (event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
		VALUES ('event-exception', 1, 'input-exception', 'eligibility-exception', '{"deviceSchedules":[{"deviceId":"device-1"},{"deviceId":"device-3"}]}', 'fallback-1', 'model-1', 'exception');
		INSERT INTO plan_versions (event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
		VALUES ('event-exception', 2, 'input-exception', 'eligibility-exception', '{"deviceSchedules":[{"deviceId":"device-2"}]}', 'fallback-1', 'model-1', 'exception');
		INSERT INTO command_intents (command_id, idempotency_key, device_id, event_id, plan_version, generation, setpoint_kw, issued_at, effective_at, expires_at, policy_version, correlation_id)
		VALUES ('command-1', 'key-command-1', 'device-1', 'event-exception', 1, 5, 10, $1, $1, $1::timestamptz + interval '1 hour', 'policy-1', 'exception');
		INSERT INTO command_intents (command_id, idempotency_key, device_id, event_id, plan_version, generation, setpoint_kw, issued_at, effective_at, expires_at, policy_version, correlation_id)
		VALUES ('command-2', 'key-command-2', 'device-2', 'event-exception', 2, 6, 10, $1::timestamptz + interval '5 minutes', $1::timestamptz + interval '5 minutes', $1::timestamptz + interval '1 hour', 'policy-1', 'exception');
		INSERT INTO command_intents (command_id, idempotency_key, device_id, event_id, plan_version, generation, setpoint_kw, issued_at, effective_at, expires_at, policy_version, correlation_id)
		VALUES ('command-3', 'key-command-3', 'device-2', 'event-exception', 2, 7, 0, $1::timestamptz + interval '6 minutes', $1::timestamptz + interval '6 minutes', $1::timestamptz + interval '1 hour', 'policy-1', 'exception');
		INSERT INTO command_intents (command_id, idempotency_key, device_id, event_id, plan_version, generation, setpoint_kw, issued_at, effective_at, expires_at, policy_version, correlation_id)
		VALUES ('command-4', 'key-command-4', 'device-3', 'event-exception', 1, 8, 10, $1, $1, $1::timestamptz + interval '1 hour', 'policy-1', 'exception');
		INSERT INTO command_states (command_id, state, recorded_at, correlation_id)
		VALUES ('command-1', 'SENT', $1, 'exception'), ('command-1', 'UNCERTAIN', $1::timestamptz + interval '2 minutes', 'exception'), ('command-1', 'ACKNOWLEDGED', $1::timestamptz + interval '4 minutes', 'exception'),
		('command-4', 'REJECTED', $1::timestamptz + interval '2 minutes', 'exception');
		INSERT INTO command_acknowledgements (acknowledgement_id, command_id, idempotency_key, receipt_status, received_at, gateway_id, correlation_id)
		VALUES ('ack-1', 'command-1', 'ack-key-1', 'ACCEPTED', $1::timestamptz + interval '3 minutes', 'gateway-1', 'exception');
		INSERT INTO command_outbox (command_id, state, attempts, published_at, correlation_id)
		VALUES ('command-1', 'PUBLISHED', 2, $1::timestamptz + interval '1 minute', 'exception');
		INSERT INTO telemetry_observations (observed_at, device_id, sequence, observation_id, payload)
		VALUES ($1::timestamptz + interval '1 minute', 'device-1', 1, 'observation-1', '{"valueState":"VALUE_STATE_MISSING"}'),
		($1::timestamptz - interval '1 minute', 'device-1', 2, 'observation-before', '{"valueState":"VALUE_STATE_MISSING"}');
		INSERT INTO audit_journal (occurred_at, actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1::timestamptz + interval '5 minutes', 'decision', 'REPLACEMENT_PLANNED', 'event', 'event-exception', '{"plan_version":2}', 'exception')`, pgx.QueryExecModeSimpleProtocol, begin)
	if err != nil {
		t.Fatal(err)
	}
	source := NewPostgresSource(pool, nil, nil, func() time.Time { return begin.Add(6 * time.Minute) }, time.Minute)
	service := NewService(source, time.Second)
	request := connect.NewRequest(&gridosv1.GetEventTimelineRequest{EventId: "event-exception"})
	request.Header().Set("X-GridOS-Role", "operator")
	response, err := service.GetEventTimeline(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	assertExceptionEvidence(t, response.Msg.GetExceptions())
	snapshot, err := source.Snapshot(ctx, "event-exception")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.GetExceptions()) != len(response.Msg.GetExceptions()) {
		t.Fatalf("watch exceptions = %d, timeline exceptions = %d", len(snapshot.GetExceptions()), len(response.Msg.GetExceptions()))
	}
}

func assertExceptionEvidence(t *testing.T, exceptions []*gridosv1.EventException) {
	t.Helper()
	require.Contains(t, gridosv1.EventExceptionKind_value, "EVENT_EXCEPTION_KIND_REJECTED_COMMAND")
	rejected := gridosv1.EventExceptionKind_value["EVENT_EXCEPTION_KIND_REJECTED_COMMAND"]
	want := map[gridosv1.EventExceptionKind]string{
		gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_MISSING_TELEMETRY:      "observation-1",
		gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_UNCERTAIN_COMMAND:      "command-1",
		gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_LATE_ACCEPTANCE:        "ack-1",
		gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_COMMAND_RETRY:          "command-1",
		gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_REPLACEMENT_PLANNED:    "",
		gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_STALE_CAPACITY_REMOVED: "",
		gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_REBALANCED_COMMAND:     "command-2",
		gridosv1.EventExceptionKind(rejected):                                   "command-4",
	}
	for _, exception := range exceptions {
		if exception.GetKind() == gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_STALE_CAPACITY_REMOVED && exception.GetDeviceId() == "device-3" {
			t.Fatal("device without missing telemetry was classified as stale")
		}
		if exception.GetKind() == gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_REBALANCED_COMMAND && exception.GetCommandId() == "command-3" {
			t.Fatal("zero-setpoint stop was classified as a rebalance")
		}
		if exception.GetEventId() != "event-exception" || exception.GetOccurredAt() == nil || exception.GetEvidenceId() == "" {
			t.Fatalf("incomplete exception: %v", exception)
		}
		if exception.GetKind() == gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_STALE_CAPACITY_REMOVED && exception.GetDeviceId() != "device-1" || exception.GetKind() == gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_REBALANCED_COMMAND && exception.GetDeviceId() != "device-2" {
			t.Fatalf("wrong recovery device: %v", exception)
		}
		if evidence, ok := want[exception.GetKind()]; ok {
			if evidence != "" && exception.GetEvidenceId() != evidence {
				t.Fatalf("exception %s evidence = %s, want %s", exception.GetKind(), exception.GetEvidenceId(), evidence)
			}
			delete(want, exception.GetKind())
		}
		if exception.GetEvidenceId() == "observation-before" {
			t.Fatal("observation before the event window was included")
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing exception kinds: %v", want)
	}
}

func exceptionDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	adminURL := os.Getenv("GRIDOS_DATABASE_URL")
	if adminURL == "" {
		adminURL = "postgres://gridos:gridos@127.0.0.1:5432/gridos?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("gridos_events_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+identifier+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})
	config, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate migrations")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(currentFile), "../../../../../database/migrations/*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, path := range files {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := pool.Exec(ctx, string(contents)); execErr != nil {
			t.Fatalf("apply %s: %v", filepath.Base(path), execErr)
		}
	}
	return pool
}

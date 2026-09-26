package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresEventStoreSurvivesServiceRestart(t *testing.T) {
	pool := apiTestDatabase(t)
	seedAPIEvent(t, pool)
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	service := NewService(NewPostgresEventStore(pool), fleet.NewTwin(time.Minute), nil, func() time.Time { return now })
	service.approveWorkflow = func(context.Context, string, dispatchWorkflowApproval) error { return nil }
	service.launchWorkflow = func(context.Context, string, *gridosv1.LaunchEventRequest) error { return nil }
	first := httptest.NewServer(NewHandler(service))
	client := gridosv1connect.NewDispatchServiceClient(http.DefaultClient, first.URL)
	approval := connect.NewRequest(&gridosv1.ApproveEventRequest{EventId: "event-restart", PlanVersion: 3, IdempotencyKey: "approve-restart", ApprovedBy: "approver-1", ApprovedAt: timestamp(now)})
	approval.Header().Set(roleHeader, "approver")
	if _, err := client.ApproveEvent(context.Background(), approval); err != nil {
		t.Fatal(err)
	}
	launch := connect.NewRequest(&gridosv1.LaunchEventRequest{EventId: "event-restart", PlanVersion: 3, IdempotencyKey: "launch-restart", RequestedBy: "approver-1", RequestedAt: timestamp(now.Add(time.Second))})
	launch.Header().Set(roleHeader, "approver")
	if _, err := client.LaunchEvent(context.Background(), launch); err != nil {
		t.Fatal(err)
	}
	first.Close()

	second := httptest.NewServer(NewHandler(NewService(NewPostgresEventStore(pool), fleet.NewTwin(time.Minute), nil, func() time.Time { return now.Add(time.Minute) })))
	defer second.Close()
	restarted := gridosv1connect.NewDispatchServiceClient(http.DefaultClient, second.URL)
	get := connect.NewRequest(&gridosv1.GetEventRequest{EventId: "event-restart"})
	get.Header().Set(roleHeader, "operator")
	response, err := restarted.GetEvent(context.Background(), get)
	if err != nil {
		t.Fatal(err)
	}
	event := response.Msg.GetEvent()
	if event.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_APPROVED || event.GetLaunch() != nil {
		t.Fatalf("restarted event = %#v", event)
	}
	groups := response.Msg.GetExclusions()
	if len(groups) != 2 || groups[0].GetCount() != 2 || groups[1].GetCount() != 1 {
		t.Fatalf("grouped exclusions = %#v", groups)
	}
	var approvals, launches int
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM operator_approvals WHERE event_id = $1", "event-restart").Scan(&approvals); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM audit_journal WHERE resource_id = $1 AND action = 'EVENT_LAUNCHED'", "event-restart").Scan(&launches); err != nil {
		t.Fatal(err)
	}
	if approvals != 1 || launches != 0 {
		t.Fatalf("approvals = %d, launch audits = %d", approvals, launches)
	}
}

func apiTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	adminURL := os.Getenv("GRIDOS_DATABASE_URL")
	if adminURL == "" {
		adminURL = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("gridos_api_%d", time.Now().UnixNano())
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
	applyAPIMigrations(t, pool)
	return pool
}

func applyAPIMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(file), "../../../../database/migrations/*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, path := range files {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := pool.Exec(context.Background(), string(contents)); execErr != nil {
			t.Fatalf("apply %s: %v", filepath.Base(path), execErr)
		}
	}
}

func seedAPIEvent(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO dispatch_requests
        (request_id, event_type, begin_time, end_time, target_kw, measurement_boundary, load_zones, correlation_id)
        VALUES ('request-restart', 'GRID_SERVICE', now(), now() + interval '1 hour', 100, 'METER_NET_EXPORT', ARRAY['LZ_AEN'], 'restart');
        INSERT INTO dispatch_events (event_id, request_id, state, plan_version, correlation_id)
        VALUES ('event-restart', 'request-restart', 'VALIDATED', 3, 'restart');
        INSERT INTO input_snapshots (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
        VALUES ('input-restart', 'event-restart', now(), '{}', '{}', 'restart');
        INSERT INTO eligibility_snapshots (snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
        VALUES ('eligibility-restart', 'event-restart', now(), ARRAY['device-1'], '[{"reason":"EXCLUSION_REASON_RESERVE"},{"reason":"EXCLUSION_REASON_RESERVE"},{"reason":"EXCLUSION_REASON_STALE_TELEMETRY"}]', 'policy-1', 'restart');
        INSERT INTO plan_versions (event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
        VALUES ('event-restart', 3, 'input-restart', 'eligibility-restart', '{}', 'fallback-1', 'model-1', 'restart')`)
	if err != nil {
		t.Fatal(err)
	}
}

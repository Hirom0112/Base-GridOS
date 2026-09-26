package control_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/dispatch"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type restartActivities struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func TestWorkerRestart(t *testing.T) {
	if os.Getenv("GRIDOS_WORKER_HELPER") != "" {
		runRestartWorker(t)
		return
	}
	pool, databaseURL := restartDatabase(t)
	now := time.Now().UTC()
	request := &gridosv1.EventRequest{
		RequestId: "restart-event", EventType: "GRID_SERVICE", BeginTime: timestamppb.New(now), EndTime: timestamppb.New(now.Add(time.Hour)),
		TargetKw: 1, MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, LoadZones: []string{}, CorrelationId: "restart",
	}
	_, err := storage.NewPostgresEventStore(pool).Create(context.Background(), request, "restart-create", now)
	if err != nil {
		t.Fatal(err)
	}
	taskQueue := fmt.Sprintf("worker-restart-%d", time.Now().UnixNano())
	first := startRestartWorker(t, databaseURL, taskQueue)
	temporalClient, err := client.Dial(client.Options{HostPort: "127.0.0.1:7233"})
	if err != nil {
		t.Fatal(err)
	}
	defer temporalClient.Close()
	run, err := temporalClient.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{ID: taskQueue, TaskQueue: taskQueue}, dispatch.Workflow, dispatch.Input{
		EventID: "restart-event", Generation: 1, AcknowledgementDeadline: now.Add(2 * time.Second), Request: request,
	})
	if err != nil {
		t.Fatal(err)
	}
	waitRestartState(t, pool, "VALIDATED")
	advanceRestartEvent(t, pool, "VALIDATED", "APPROVED")
	if err = temporalClient.SignalWorkflow(context.Background(), taskQueue, run.GetRunID(), dispatch.ApproveEventSignal, dispatch.Approval{ApprovedBy: "operator-1"}); err != nil {
		t.Fatal(err)
	}
	waitRestartState(t, pool, "SENT")
	stopRestartWorker(t, first)
	second := startRestartWorker(t, databaseURL, taskQueue)
	defer stopRestartWorker(t, second)
	waitRestartState(t, pool, "VERIFIED")
	var intents int
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM command_intents WHERE event_id = 'restart-event'").Scan(&intents); err != nil {
		t.Fatal(err)
	}
	if intents != 1 {
		t.Fatalf("physical intents = %d, want 1", intents)
	}
	_ = temporalClient.CancelWorkflow(context.Background(), taskQueue, run.GetRunID())
}

func (activities *restartActivities) FreezeInputs(_ context.Context, input dispatch.Input) (dispatch.FrozenEvent, error) {
	return dispatch.FrozenEvent{Input: input}, nil
}

func (activities *restartActivities) RequestPlan(ctx context.Context, frozen dispatch.FrozenEvent) (dispatch.FrozenEvent, error) {
	now := activities.now()
	_, err := activities.pool.Exec(ctx, `INSERT INTO input_snapshots (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
		VALUES ('restart-input', $1, $2, '{}', '{"source":"SIMULATED"}', 'restart')`, frozen.Input.EventID, now)
	if err == nil {
		_, err = activities.pool.Exec(ctx, `INSERT INTO eligibility_snapshots (snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
			VALUES ('restart-eligibility', $1, $2, '{}', '[]', 'policy-1', 'restart')`, frozen.Input.EventID, now)
	}
	if err == nil {
		_, err = activities.pool.Exec(ctx, `INSERT INTO plan_versions (event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id, created_at)
			VALUES ($1, 1, 'restart-input', 'restart-eligibility', '{}', 'test', 'test', 'restart', $2)`, frozen.Input.EventID, now)
	}
	if err != nil {
		return frozen, err
	}
	advance := storage.EventTransition{EventID: frozen.Input.EventID, ExpectedState: "REQUESTED", NextState: "PLANNED", ActorID: "worker", CorrelationID: "restart", OccurredAt: activities.now()}
	_, err = storage.TransitionEvent(ctx, activities.pool, advance)
	frozen.Input.PlanVersion = 1
	return frozen, err
}

func (activities *restartActivities) ValidatePlan(ctx context.Context, frozen dispatch.FrozenEvent) error {
	_, err := storage.TransitionEvent(ctx, activities.pool, storage.EventTransition{EventID: frozen.Input.EventID, ExpectedState: "PLANNED", NextState: "VALIDATED", ActorID: "worker", CorrelationID: "restart", OccurredAt: activities.now()})
	return err
}

func (activities *restartActivities) PersistIntents(ctx context.Context, input dispatch.Input) error {
	now := activities.now()
	err := storage.InsertCommand(ctx, activities.pool, storage.CommandIntent{
		CommandID: "restart-command", IdempotencyKey: "restart-command", DeviceID: "device-1", EventID: input.EventID,
		PlanVersion: 1, Generation: 1, SetpointKW: 1, IssuedAt: now, EffectiveAt: now, ExpiresAt: now.Add(time.Hour), PolicyVersion: "policy-1", CorrelationID: "restart",
	})
	if err != nil {
		return err
	}
	_, err = storage.TransitionEvent(ctx, activities.pool, storage.EventTransition{EventID: input.EventID, ExpectedState: "APPROVED", NextState: "COMMANDS_PERSISTED", ActorID: "worker", CorrelationID: "restart", OccurredAt: now})
	return err
}

func (activities *restartActivities) PublishCommands(ctx context.Context, input dispatch.Input) error {
	now := activities.now()
	if _, err := storage.TransitionCommand(ctx, activities.pool, storage.CommandTransition{CommandID: "restart-command", ExpectedState: "PERSISTED", NextState: "SENT", OccurredAt: now, CorrelationID: "restart"}); err != nil {
		return err
	}
	_, err := storage.TransitionEvent(ctx, activities.pool, storage.EventTransition{EventID: input.EventID, ExpectedState: "COMMANDS_PERSISTED", NextState: "SENT", ActorID: "worker", CorrelationID: "restart", OccurredAt: now})
	return err
}

func (activities *restartActivities) TrackAcknowledgements(ctx context.Context, input dispatch.Input) error {
	_, err := storage.TransitionEvent(ctx, activities.pool, storage.EventTransition{EventID: input.EventID, ExpectedState: "SENT", NextState: "ACKNOWLEDGED_OR_UNCERTAIN", ActorID: "worker", CorrelationID: "restart", OccurredAt: activities.now()})
	return err
}

func (activities *restartActivities) VerifyDelivery(ctx context.Context, input dispatch.Input) error {
	now := activities.now()
	if _, err := storage.TransitionEvent(ctx, activities.pool, storage.EventTransition{EventID: input.EventID, ExpectedState: "ACKNOWLEDGED_OR_UNCERTAIN", NextState: "EXECUTING", ActorID: "worker", CorrelationID: "restart", OccurredAt: now}); err != nil {
		return err
	}
	_, err := storage.TransitionEvent(ctx, activities.pool, storage.EventTransition{EventID: input.EventID, ExpectedState: "EXECUTING", NextState: "VERIFIED", ActorID: "worker", CorrelationID: "restart", OccurredAt: now})
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(3 * time.Second):
		return nil
	}
}

func (activities *restartActivities) EndEvent(context.Context, dispatch.Input) error { return nil }

func (activities *restartActivities) ReconcileLateMessages(context.Context, dispatch.Input) error {
	return nil
}

func (activities *restartActivities) ProduceReport(context.Context, dispatch.Input) error { return nil }

func runRestartWorker(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), os.Getenv("GRIDOS_RESTART_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	temporalClient, err := client.Dial(client.Options{HostPort: "127.0.0.1:7233"})
	if err != nil {
		t.Fatal(err)
	}
	defer temporalClient.Close()
	restartWorker := worker.New(temporalClient, os.Getenv("GRIDOS_TASK_QUEUE"), worker.Options{})
	restartWorker.RegisterWorkflow(dispatch.Workflow)
	restartWorker.RegisterActivity(&restartActivities{pool: pool, now: time.Now})
	if err = restartWorker.Run(worker.InterruptCh()); err != nil {
		t.Fatal(err)
	}
}

func startRestartWorker(t *testing.T, databaseURL, taskQueue string) *exec.Cmd {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "-test.run=TestWorkerRestart", "-test.v")
	command.Env = append(os.Environ(), "GRIDOS_WORKER_HELPER=1", "GRIDOS_RESTART_DATABASE_URL="+databaseURL, "GRIDOS_TASK_QUEUE="+taskQueue)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	return command
}

func stopRestartWorker(t *testing.T, command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Fatal(err)
	}
	_, _ = command.Process.Wait()
}

func waitRestartState(t *testing.T, pool *pgxpool.Pool, expected string) {
	deadline := time.Now().Add(25 * time.Second)
	actual := ""
	for time.Now().Before(deadline) {
		if pool.QueryRow(context.Background(), "SELECT state FROM dispatch_events WHERE event_id = 'restart-event'").Scan(&actual) == nil && actual == expected {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("event state = %s, want %s", actual, expected)
}

func advanceRestartEvent(t *testing.T, pool *pgxpool.Pool, expected, next string) {
	changed, err := storage.TransitionEvent(context.Background(), pool, storage.EventTransition{EventID: "restart-event", ExpectedState: expected, NextState: next, ActorID: "operator", CorrelationID: "restart", OccurredAt: time.Now()})
	if err != nil || !changed {
		t.Fatalf("advance event: %v, changed %t", err, changed)
	}
}

func restartDatabase(t *testing.T) (*pgxpool.Pool, string) {
	ctx := context.Background()
	adminURL := os.Getenv("GRIDOS_DATABASE_URL")
	if adminURL == "" {
		adminURL = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("gridos_restart_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
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
	_, file, _, _ := runtime.Caller(0)
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(file), "../../../database/migrations/*.sql"))
	sort.Strings(files)
	for _, path := range files {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = pool.Exec(ctx, string(contents)); err != nil {
			t.Fatal(err)
		}
	}
	return pool, fmt.Sprintf("postgres://gridos:gridos@localhost:5432/%s?sslmode=disable", name)
}

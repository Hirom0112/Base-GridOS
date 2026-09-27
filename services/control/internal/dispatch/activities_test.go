package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	controlapi "github.com/Hirom0112/Base-GridOS/services/control/internal/api"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/replay"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/safety"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type activitySnapshotter struct {
	snapshot controlapi.FrozenSnapshot
}

func (snapshotter activitySnapshotter) Freeze(context.Context, *gridosv1.DispatchEvent, *gridosv1.EventRequest) (controlapi.FrozenSnapshot, error) {
	return snapshotter.snapshot, nil
}

type activityOptimizer struct {
	plan            *gridosv1.DispatchPlan
	replacementPlan *gridosv1.DispatchPlan
}

func (optimizer activityOptimizer) Optimize(context.Context, *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error) {
	return optimizer.plan, nil
}

func (activityOptimizer) Forecast(context.Context, *gridosv1.ForecastRequest) (*gridosv1.ForecastResponse, error) {
	return &gridosv1.ForecastResponse{}, nil
}

func (optimizer activityOptimizer) Replace(context.Context, *gridosv1.ReplaceRequest) (*gridosv1.ReplaceResponse, error) {
	return &gridosv1.ReplaceResponse{ReplacementPlan: optimizer.replacementPlan}, nil
}

type activitySafety struct{}

func (activitySafety) Validate(*gridosv1.DispatchPlan, safety.CanonicalState) error { return nil }

type activityPublisher struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func (publisher *activityPublisher) PublishBatch(ctx context.Context) error {
	commands, err := storage.ClaimOutbox(ctx, publisher.pool, storage.OutboxClaim{AvailableAt: publisher.now(), LeaseUntil: publisher.now().Add(time.Minute), BatchSize: 100})
	if err != nil {
		return err
	}
	for _, command := range commands {
		if err = publisher.Publish(ctx, command); err != nil {
			return err
		}
	}
	return nil
}

func (publisher *activityPublisher) Publish(ctx context.Context, command storage.ClaimedCommand) error {
	if _, err := storage.TransitionCommand(ctx, publisher.pool, storage.CommandTransition{CommandID: command.CommandID, ExpectedState: "PERSISTED", NextState: "SENT", OccurredAt: publisher.now(), CorrelationID: command.CorrelationID}); err != nil {
		return err
	}
	if err := storage.RecordAcknowledgement(ctx, publisher.pool, storage.Acknowledgement{
		AcknowledgementID: command.CommandID + "-ack", CommandID: command.CommandID, IdempotencyKey: command.IdempotencyKey,
		ReceiptStatus: "ACCEPTED", ReceivedAt: publisher.now(), GatewayID: "gateway-1", CorrelationID: command.CorrelationID,
	}); err != nil {
		return err
	}
	return storage.MarkOutboxPublished(ctx, publisher.pool, command.CommandID, publisher.now())
}

type activityHarness struct {
	activities *Activities
	events     *controlapi.PostgresEventStore
	input      Input
	pool       *pgxpool.Pool
}

func TestFreezeInputsActivity(t *testing.T) {
	harness := newActivityHarness(t)
	fleetFile := filepath.Join(t.TempDir(), "fleet.jsonl")
	require.NoError(t, os.WriteFile(fleetFile, []byte("simulated fleet"), 0o600))
	harness.activities.ReplayDirectory = t.TempDir()
	harness.activities.ReplayInput = replay.Input{Seed: 42, FleetFile: fleetFile, SolverVersion: "highs", FallbackVersion: "fallback-1", CodeVersion: "test"}
	frozen, err := harness.activities.FreezeInputs(context.Background(), harness.input)
	require.NoError(t, err)
	require.Equal(t, harness.input.EventID+"-input-1", frozen.InputSnapshotID)
	require.Equal(t, harness.input.EventID+"-eligibility-1", frozen.EligibilitySnapshotID)
	manifest, err := replay.Load(harness.activities.ReplayDirectory, harness.input.EventID)
	require.NoError(t, err)
	require.Equal(t, frozen.InputSnapshotID, manifest.InputSnapshotID)
	require.Equal(t, frozen.EligibilitySnapshotID, manifest.EligibilitySnapshotID)
	require.Equal(t, "policy-1", manifest.PolicyVersion)
}

func TestFreezeInputsAustinResultStaysBelowTemporalLimit(t *testing.T) {
	pool := activityDatabase(t)
	now := time.Now().UTC()
	twin := fleet.NewTwin(time.Minute)
	sites, _, err := fleet.Load(filepath.Join(activityTestRoot(t), "testdata/fleets/austin-5000.jsonl"), twin, now)
	require.NoError(t, err)
	events := controlapi.NewPostgresEventStore(pool)
	request := &gridosv1.EventRequest{
		RequestId: "austin-freeze", EventType: "GRID_SERVICE", BeginTime: timestamppb.New(now.Add(time.Minute)), EndTime: timestamppb.New(now.Add(time.Hour)),
		TargetKw: 1, MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, LoadZones: []string{"LZ_AEN"}, CorrelationId: "austin-freeze",
	}
	_, err = events.Create(context.Background(), request, "create-austin-freeze", now)
	require.NoError(t, err)
	activities := &Activities{Dispatcher: &controlapi.Dispatcher{Events: events, Snapshots: controlapi.NewFleetSnapshotter(twin, sites, func() time.Time { return now }), Optimizer: activityOptimizer{}}, Events: events, Pool: pool, Now: func() time.Time { return now }}
	frozen, err := activities.FreezeInputs(context.Background(), Input{EventID: request.GetRequestId(), Request: request})
	require.NoError(t, err)
	payload, err := json.Marshal(frozen)
	require.NoError(t, err)
	require.Less(t, len(payload), 64*1024)
}

func TestRequestPlanActivity(t *testing.T) {
	harness := newActivityHarness(t)
	frozen := harness.freeze(t)
	planned, err := harness.activities.RequestPlan(context.Background(), frozen)
	require.NoError(t, err)
	require.Equal(t, uint64(1), planned.Input.PlanVersion)
	require.Equal(t, 1, harness.count(t, "plan_versions"))
	require.Equal(t, 1, harness.count(t, "input_snapshots"))
	require.Equal(t, 1, harness.count(t, "eligibility_snapshots"))
}

func TestValidatePlanActivity(t *testing.T) {
	harness := newActivityHarness(t)
	frozen := harness.plan(t)
	require.NoError(t, harness.activities.ValidatePlan(context.Background(), frozen))
	require.Equal(t, "VALIDATED", harness.state(t))
}

func TestPersistIntentsActivity(t *testing.T) {
	harness := newActivityHarness(t)
	harness.approve(t)
	require.NoError(t, harness.activities.PersistIntents(context.Background(), PersistInput{Input: harness.input, Launch: harness.launch()}))
	require.Equal(t, 1, harness.count(t, "command_intents"))
	require.Equal(t, 1, harness.count(t, "command_outbox"))
	require.Equal(t, "COMMANDS_PERSISTED", harness.state(t))
}

func TestPublishCommandsActivity(t *testing.T) {
	harness := newActivityHarness(t)
	harness.persist(t)
	require.NoError(t, harness.activities.PublishCommands(context.Background(), harness.input))
	require.Equal(t, "SENT", harness.state(t))
	require.Equal(t, "ACKNOWLEDGED", harness.commandState(t))
}

func TestPublishCommandsDrains113Intents(t *testing.T) {
	harness := newActivityHarness(t)
	harness.persist(t)
	for index := 1; index < 113; index++ {
		command := storage.CommandIntent{
			CommandID: fmt.Sprintf("event-1-extra-%03d", index), IdempotencyKey: fmt.Sprintf("event-1-extra-%03d", index),
			DeviceID: "device-1", EventID: harness.input.EventID, PlanVersion: 1, Generation: 1,
			SetpointKW: 1, IssuedAt: harness.activities.Now(), EffectiveAt: harness.input.Request.GetBeginTime().AsTime(),
			ExpiresAt: harness.input.Request.GetEndTime().AsTime(), PolicyVersion: "policy-1", CorrelationID: "correlation-1",
		}
		require.NoError(t, storage.InsertCommand(context.Background(), harness.pool, command))
	}
	require.NoError(t, harness.activities.PublishCommands(context.Background(), harness.input))
	require.NoError(t, harness.activities.TrackAcknowledgements(context.Background(), harness.input))
	require.Equal(t, "ACKNOWLEDGED_OR_UNCERTAIN", harness.state(t))
}

func TestTrackAcknowledgementsActivity(t *testing.T) {
	harness := newActivityHarness(t)
	harness.publish(t)
	require.NoError(t, harness.activities.TrackAcknowledgements(context.Background(), harness.input))
	require.Equal(t, "ACKNOWLEDGED_OR_UNCERTAIN", harness.state(t))
}

func TestEndEventActivity(t *testing.T) {
	harness := newActivityHarness(t)
	harness.persist(t)
	harness.liveClock()
	require.NoError(t, harness.activities.EndEvent(context.Background(), harness.input))
	require.Equal(t, 2, harness.count(t, "command_intents"))
	var setpoint float64
	require.NoError(t, harness.pool.QueryRow(context.Background(), "SELECT setpoint_kw FROM command_intents ORDER BY generation DESC LIMIT 1").Scan(&setpoint))
	require.Zero(t, setpoint)
}

func TestEndEventRetryDoesNotDuplicateZeroCommands(t *testing.T) {
	harness := newActivityHarness(t)
	harness.persist(t)
	harness.liveClock()
	require.NoError(t, harness.activities.EndEvent(context.Background(), harness.input))
	require.NoError(t, harness.activities.EndEvent(context.Background(), harness.input))
	require.Equal(t, 2, harness.count(t, "command_intents"))
}

func TestEndEventAfterDispatchWindow(t *testing.T) {
	harness := newActivityHarness(t)
	harness.persist(t)
	harness.liveClock()
	require.NoError(t, harness.activities.EndEvent(context.Background(), harness.input))
	require.Equal(t, 2, harness.count(t, "command_intents"))
}

func TestEmergencyStopIssuesPerDeviceZeroSetpoints(t *testing.T) {
	harness := newActivityHarness(t)
	harness.persist(t)
	harness.liveClock()
	require.NoError(t, harness.activities.IssueEmergencyStop(context.Background(), EmergencyCommand{EventID: harness.input.EventID, Generation: 2}))
	var deviceID string
	var setpoint float64
	require.NoError(t, harness.pool.QueryRow(context.Background(), "SELECT device_id, setpoint_kw FROM command_intents ORDER BY generation DESC LIMIT 1").Scan(&deviceID, &setpoint))
	require.Equal(t, "device-1", deviceID)
	require.Zero(t, setpoint)
}

func TestEmergencyStopDrainsAllZeroCommands(t *testing.T) {
	harness := newActivityHarness(t)
	harness.plan(t)
	harness.liveClock()
	ctx := context.Background()
	now := harness.activities.Now()
	for index := range 166 {
		id := fmt.Sprintf("prestop-%03d", index)
		require.NoError(t, storage.InsertCommand(ctx, harness.pool, storage.CommandIntent{
			CommandID: id, IdempotencyKey: id, DeviceID: fmt.Sprintf("stop-device-%03d", index), EventID: harness.input.EventID,
			PlanVersion: 1, SetpointKW: 1, IssuedAt: now, EffectiveAt: now.Add(time.Minute), ExpiresAt: now.Add(time.Hour),
			PolicyVersion: "policy-1", CorrelationID: "correlation-1",
		}))
	}
	for range 2 {
		require.NoError(t, harness.activities.Dispatcher.Publish(ctx, nil))
	}
	var accepted int
	require.NoError(t, harness.pool.QueryRow(ctx, `SELECT count(*) FROM command_intents intent
		JOIN command_acknowledgements ack USING (command_id)
		WHERE intent.event_id = $1 AND intent.setpoint_kw <> 0 AND ack.receipt_status = 'ACCEPTED'`, harness.input.EventID).Scan(&accepted))
	require.Equal(t, 166, accepted)
	require.NoError(t, harness.activities.IssueEmergencyStop(ctx, EmergencyCommand{EventID: harness.input.EventID, Generation: 2}))
	require.NoError(t, harness.pool.QueryRow(ctx, `SELECT count(*) FROM command_intents intent
		JOIN command_acknowledgements ack USING (command_id)
		WHERE intent.event_id = $1 AND intent.setpoint_kw = 0 AND ack.receipt_status = 'ACCEPTED'`, harness.input.EventID).Scan(&accepted))
	require.Equal(t, 166, accepted)
}

func TestProduceReportActivity(t *testing.T) {
	harness := newActivityHarness(t)
	harness.track(t)
	harness.advance(t, "ACKNOWLEDGED_OR_UNCERTAIN", "EXECUTING")
	harness.advance(t, "EXECUTING", "VERIFIED")
	harness.advance(t, "VERIFIED", "RECONCILED")
	require.NoError(t, harness.activities.ProduceReport(context.Background(), harness.input))
	require.Equal(t, "REPORTED", harness.state(t))
}

func (harness *activityHarness) advance(t *testing.T, expected, next string) {
	_, err := harness.events.Advance(context.Background(), harness.input.EventID, expected, next, "test", time.Now())
	require.NoError(t, err)
}

func TestIssueReplacementActivity(t *testing.T) {
	harness := newActivityHarness(t)
	harness.persist(t)
	require.Error(t, harness.activities.IssueReplacement(context.Background(), ReplacementCommand{EventID: "event-1", Request: harness.input.Request, DroppedDeviceIDs: []string{"device-1"}, EnvelopeDeviceIDs: []string{"device-1", "device-2"}, Generation: 2}))
	require.Equal(t, 1, harness.count(t, "command_intents"))
}

func (harness *activityHarness) liveClock() {
	harness.activities.Now = time.Now
	harness.activities.Dispatcher.Commands = controlapi.NewCommandPipeline(harness.pool, &activityPublisher{pool: harness.pool, now: time.Now})
}

func newActivityHarness(t *testing.T) *activityHarness {
	pool := activityDatabase(t)
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	begin := now.Add(time.Minute)
	request := &gridosv1.EventRequest{
		RequestId: "event-1", EventType: "GRID_SERVICE", BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(begin.Add(time.Hour)),
		TargetKw: 1, MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, LoadZones: []string{}, CorrelationId: "correlation-1",
	}
	events := controlapi.NewPostgresEventStore(pool)
	_, err := events.Create(context.Background(), request, "create-event-1", now)
	require.NoError(t, err)
	energy := 5.0
	snapshot := controlapi.FrozenSnapshot{
		Optimization: &gridosv1.OptimizationRequest{
			EventId: "event-1", PlanVersion: 1, CorrelationId: "correlation-1", Budget: durationpb.New(5 * time.Second),
			EligibilitySnapshot: &gridosv1.EligibilitySnapshot{EventId: "event-1", EligibleDeviceIds: []string{"device-1"}},
			ReservePolicy:       &gridosv1.ReservePolicy{PolicyVersion: "policy-1"},
		},
		Canonical: safety.CanonicalState{Now: now, PolicyVersion: "policy-1", ExpectedGeneration: 1, Devices: map[string]safety.DeviceState{"device-1": {EnergyKWh: &energy}}},
	}
	plan := &gridosv1.DispatchPlan{
		EventId: "event-1", PlanVersion: 1, CreatedAt: timestamppb.New(now), SolverVersion: "solver-1", ModelVersion: "model-1",
		DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "device-1", Intervals: []*gridosv1.DeviceScheduleInterval{{BeginTime: request.BeginTime, EndTime: request.EndTime, SetpointKw: 1, ExpectedEnergyKwh: 4}}}},
	}
	publisher := &activityPublisher{pool: pool, now: func() time.Time { return now }}
	dispatcher := &controlapi.Dispatcher{
		Events: events, Snapshots: activitySnapshotter{snapshot: snapshot}, Optimizer: activityOptimizer{plan: plan}, Safety: activitySafety{},
		Approval: controlapi.NewStoredApprovalGate(events), Commands: controlapi.NewCommandPipeline(pool, publisher), Now: func() time.Time { return now },
	}
	activities := &Activities{Dispatcher: dispatcher, Events: events, Pool: pool, Reports: controlapi.NewPostgresReportSource(pool), Now: func() time.Time { return now }}
	return &activityHarness{activities: activities, events: events, input: Input{EventID: "event-1", Request: request, PlanVersion: 1, Generation: 1}, pool: pool}
}

func (harness *activityHarness) freeze(t *testing.T) FrozenEvent {
	frozen, err := harness.activities.FreezeInputs(context.Background(), harness.input)
	require.NoError(t, err)
	return frozen
}

func (harness *activityHarness) plan(t *testing.T) FrozenEvent {
	frozen, err := harness.activities.RequestPlan(context.Background(), harness.freeze(t))
	require.NoError(t, err)
	harness.input = frozen.Input
	return frozen
}

func (harness *activityHarness) approve(t *testing.T) {
	frozen := harness.plan(t)
	require.NoError(t, harness.activities.ValidatePlan(context.Background(), frozen))
	_, err := harness.events.Approve(context.Background(), &gridosv1.ApproveEventRequest{
		EventId: harness.input.EventID, PlanVersion: harness.input.PlanVersion, IdempotencyKey: "approve-event-1", ApprovedBy: "operator-1", ApprovedAt: timestamppb.New(time.Now()),
	})
	require.NoError(t, err)
}

func (harness *activityHarness) persist(t *testing.T) {
	harness.approve(t)
	require.NoError(t, harness.activities.PersistIntents(context.Background(), PersistInput{Input: harness.input, Launch: harness.launch()}))
}

func (harness *activityHarness) launch() *gridosv1.LaunchEventRequest {
	return &gridosv1.LaunchEventRequest{
		EventId: harness.input.EventID, PlanVersion: harness.input.PlanVersion, IdempotencyKey: "launch-event-1", RequestedBy: "operator-1", RequestedAt: timestamppb.Now(),
	}
}

func (harness *activityHarness) publish(t *testing.T) {
	harness.persist(t)
	require.NoError(t, harness.activities.PublishCommands(context.Background(), harness.input))
}

func (harness *activityHarness) track(t *testing.T) {
	harness.publish(t)
	require.NoError(t, harness.activities.TrackAcknowledgements(context.Background(), harness.input))
}

func (harness *activityHarness) count(t *testing.T, table string) int {
	var count int
	require.NoError(t, harness.pool.QueryRow(context.Background(), "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&count))
	return count
}

func (harness *activityHarness) state(t *testing.T) string {
	var state string
	require.NoError(t, harness.pool.QueryRow(context.Background(), "SELECT state FROM dispatch_events WHERE event_id = $1", harness.input.EventID).Scan(&state))
	return state
}

func (harness *activityHarness) commandState(t *testing.T) string {
	var state string
	require.NoError(t, harness.pool.QueryRow(context.Background(), "SELECT state FROM command_states ORDER BY recorded_at DESC LIMIT 1").Scan(&state))
	return state
}

func activityDatabase(t *testing.T) *pgxpool.Pool {
	ctx := context.Background()
	url := os.Getenv("GRIDOS_DATABASE_URL")
	if url == "" {
		url = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, url)
	require.NoError(t, err)
	name := fmt.Sprintf("gridos_dispatch_%d", time.Now().UnixNano())
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})
	config, err := pgxpool.ParseConfig(url)
	require.NoError(t, err)
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	files, err := filepath.Glob(filepath.Join(activityTestRoot(t), "database/migrations/*.sql"))
	require.NoError(t, err)
	sort.Strings(files)
	for _, path := range files {
		contents, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		_, execErr := pool.Exec(ctx, string(contents))
		require.NoError(t, execErr)
	}
	return pool
}

func activityTestRoot(t *testing.T) string {
	t.Helper()
	if directory := os.Getenv("TEST_SRCDIR"); directory != "" {
		return filepath.Join(directory, os.Getenv("TEST_WORKSPACE"))
	}
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
}

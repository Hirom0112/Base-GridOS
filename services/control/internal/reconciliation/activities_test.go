package reconciliation

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type activityHarness struct {
	pool       *pgxpool.Pool
	activities *Activities
	begin      time.Time
	end        time.Time
	now        time.Time
}

func TestVerifyDeliveryActivityWritesSummariesAndUncertainty(t *testing.T) {
	harness := newActivityHarness(t)
	harness.observeExport(t, 0, 10, 20, 45, 50, 55, 60, 62)

	if err := harness.activities.VerifyDelivery(context.Background(), Input{EventID: "event-1"}); err != nil {
		t.Fatal(err)
	}
	if state := harness.eventState(t); state != "VERIFIED" {
		t.Fatalf("event state = %s, want VERIFIED", state)
	}
	if rows := harness.count(t, "verification_summaries"); rows != 12 {
		t.Fatalf("verification summaries = %d, want 12", rows)
	}
	if deliveredKW, confidence := harness.summary(t, harness.begin); deliveredKW != 4 || confidence != 0.5 {
		t.Fatalf("first interval delivered = %v kW at confidence %v, want 4 kW at 0.5", deliveredKW, confidence)
	}
	if deliveredKW, confidence := harness.summary(t, harness.begin.Add(30*time.Minute)); deliveredKW != 0 || confidence != 0 {
		t.Fatalf("gap interval delivered = %v kW at confidence %v, want 0 kW at 0", deliveredKW, confidence)
	}

	delivered, err := LoadDelivered(context.Background(), harness.pool, "event-1")
	if err != nil {
		t.Fatal(err)
	}
	if delivered == nil {
		t.Fatal("no delivered verification was recorded")
	}
	want := report.Delivered{DeliveredMWh: 4 * 35.0 / 60 / 1000, DeliveredMW: 0.004, TrackingErrorMW: 0, ResponseLatency: 0, Completeness: 35.0 / 120, Responded: 1, Commanded: 2}
	got := *delivered
	got.UncertainIntervals = nil
	if math.Abs(got.DeliveredMWh-want.DeliveredMWh) > 1e-9 || math.Abs(got.DeliveredMW-want.DeliveredMW) > 1e-9 || math.Abs(got.Completeness-want.Completeness) > 1e-9 {
		t.Fatalf("delivered = %+v, want %+v", got, want)
	}
	got.DeliveredMWh, got.DeliveredMW, got.Completeness = want.DeliveredMWh, want.DeliveredMW, want.Completeness
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("delivered = %+v, want %+v", got, want)
	}
	wantUncertain := []report.UncertainInterval{
		{DeviceID: "device-1", Begin: harness.begin.Add(20 * time.Minute), End: harness.begin.Add(45 * time.Minute)},
		{DeviceID: "device-2", Begin: harness.begin, End: harness.end},
		{DeviceID: "device-2", Begin: harness.begin.Add(time.Minute), End: harness.end, Bounds: &report.PowerBounds{LowerKW: 0, UpperKW: 3}},
	}
	if !reflect.DeepEqual(delivered.UncertainIntervals, wantUncertain) {
		t.Fatalf("uncertain intervals = %+v, want %+v", delivered.UncertainIntervals, wantUncertain)
	}
}

func TestVerifyDeliveryTracksEventWindow(t *testing.T) {
	harness := newActivityHarness(t)
	harness.now = harness.begin
	if err := harness.activities.VerifyDelivery(context.Background(), Input{EventID: "event-1"}); err != nil {
		t.Fatal(err)
	}
	if state := harness.eventState(t); state != "EXECUTING" {
		t.Fatalf("at begin state = %s, want EXECUTING", state)
	}
	harness.now = harness.begin.Add(5 * time.Minute)
	if err := harness.activities.VerifyDelivery(context.Background(), Input{EventID: "event-1"}); err != nil {
		t.Fatal(err)
	}
	if state := harness.eventState(t); state != "EXECUTING" {
		t.Fatalf("during window state = %s, want EXECUTING", state)
	}
	harness.now = harness.end
	if err := harness.activities.VerifyDelivery(context.Background(), Input{EventID: "event-1"}); err != nil {
		t.Fatal(err)
	}
	if state := harness.eventState(t); state != "VERIFIED" {
		t.Fatalf("at end state = %s, want VERIFIED", state)
	}
	var decisions int
	var intervalSeconds int
	err := harness.pool.QueryRow(context.Background(), `SELECT count(*), COALESCE(max((new_values->>'interval_seconds')::int), 0)
		FROM audit_journal WHERE resource_id = 'event-1' AND action = 'VERIFICATION_CADENCE_SELECTED'`).Scan(&decisions, &intervalSeconds)
	if err != nil {
		t.Fatal(err)
	}
	if decisions != 1 || intervalSeconds != 300 {
		t.Fatalf("cadence decisions = %d, interval seconds = %d", decisions, intervalSeconds)
	}
}

func TestReconcileLateMessagesActivityAbsorbsLateTelemetry(t *testing.T) {
	harness := newActivityHarness(t)
	harness.observeExport(t, 0, 10, 20, 45, 50, 55, 60, 62)
	if err := harness.activities.VerifyDelivery(context.Background(), Input{EventID: "event-1"}); err != nil {
		t.Fatal(err)
	}
	harness.observeExport(t, 30)

	if err := harness.activities.ReconcileLateMessages(context.Background(), Input{EventID: "event-1"}); err != nil {
		t.Fatal(err)
	}
	if state := harness.eventState(t); state != "RECONCILED" {
		t.Fatalf("event state = %s, want RECONCILED", state)
	}
	delivered, err := LoadDelivered(context.Background(), harness.pool, "event-1")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(delivered.DeliveredMWh-4*45.0/60/1000) > 1e-9 || math.Abs(delivered.Completeness-45.0/120) > 1e-9 {
		t.Fatalf("delivered after late telemetry = %+v", *delivered)
	}
	if got := delivered.UncertainIntervals[0]; got.Begin != harness.begin.Add(30*time.Minute) || got.End != harness.begin.Add(45*time.Minute) {
		t.Fatalf("device-1 gap after late telemetry = %+v", got)
	}
	if rows := harness.count(t, "verification_summaries"); rows != 12 {
		t.Fatalf("verification summaries = %d, want 12 after reconciliation", rows)
	}
}

func TestDeliveredByCellKeepsUnobservedCellsUnknown(t *testing.T) {
	harness := newActivityHarness(t)
	harness.observeExport(t, 0, 10, 20, 30, 40, 50, 60)

	cells, err := DeliveredByCell(context.Background(), harness.pool, "event-1", map[string]string{
		"device-1": "cell-a",
		"device-2": "cell-b",
	}, harness.now, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 2 || cells[0].H3Cell != "cell-a" || cells[1].H3Cell != "cell-b" {
		t.Fatalf("cells = %#v", cells)
	}
	if !cells[0].Known || math.Abs(cells[0].DeliveredMW-0.004) > 1e-9 {
		t.Fatalf("measured cell = %#v", cells[0])
	}
	if cells[1].Known {
		t.Fatalf("unobserved cell = %#v, want unknown", cells[1])
	}
	if len(cells[1].UncertainIntervals) == 0 {
		t.Fatalf("unobserved cell has no uncertainty interval: %#v", cells[1])
	}
}

func TestVerifyDeliveryActivityRejectsUnsupportedBoundary(t *testing.T) {
	harness := newActivityHarness(t)
	if _, err := harness.pool.Exec(context.Background(), "UPDATE dispatch_requests SET measurement_boundary = 'IMPORT_REDUCTION_VS_BASELINE'"); err != nil {
		t.Fatal(err)
	}
	err := harness.activities.VerifyDelivery(context.Background(), Input{EventID: "event-1"})
	if err == nil || harness.eventState(t) != "ACKNOWLEDGED_OR_UNCERTAIN" {
		t.Fatalf("error = %v, state = %s; want a rejection without a lifecycle change", err, harness.eventState(t))
	}
}

func TestDetectRecoveryUsesScheduledDevicesAndStoredEnvelope(t *testing.T) {
	harness := newActivityHarness(t)
	ctx := context.Background()
	harness.now = harness.begin.Add(2 * time.Minute)
	_, err := harness.pool.Exec(ctx, `INSERT INTO eligibility_snapshots
		(snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
		VALUES ('eligibility-2', 'event-1', $1, ARRAY['device-1','device-2','device-3'], '[]', 'policy-1', 'correlation-1');
		INSERT INTO plan_versions
		(event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
		VALUES ('event-1', 2, 'input-1', 'eligibility-2',
		'{"deviceSchedules":[{"deviceId":"device-1"},{"deviceId":"device-2"}]}', 'solver-1', 'model-1', 'correlation-1');
		UPDATE dispatch_events SET plan_version = 2, state = 'EXECUTING' WHERE event_id = 'event-1'`, pgx.QueryExecModeSimpleProtocol, harness.begin.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	_, err = harness.pool.Exec(ctx, `INSERT INTO telemetry_observations (observed_at, device_id, sequence, observation_id, payload)
		VALUES ($1, 'device-1', 1, 'missing-device-1', '{"valueState":"VALUE_STATE_MISSING"}')`, harness.begin.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	recovery, err := harness.activities.DetectRecovery(ctx, Input{EventID: "event-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(recovery.EnvelopeDeviceIDs, []string{"device-1", "device-2", "device-3"}) {
		t.Fatalf("stored eligible envelope = %v", recovery.EnvelopeDeviceIDs)
	}
	if !reflect.DeepEqual(recovery.Dropped, []RecoveryDrop{{DeviceID: "device-1", Reason: "MISSING"}, {DeviceID: "device-2", Reason: "UNCERTAIN"}}) {
		t.Fatalf("scheduled drops = %+v", recovery.Dropped)
	}
	if _, err := harness.activities.DetectRecovery(ctx, Input{EventID: "event-1"}); err != nil {
		t.Fatal(err)
	}
	var decisions int
	if err := harness.pool.QueryRow(ctx, `SELECT count(*) FROM audit_journal
		WHERE resource_id = 'event-1' AND action = 'RECOVERY_DETECTED'
		AND new_values->>'reason' IN ('MISSING', 'UNCERTAIN')`).Scan(&decisions); err != nil {
		t.Fatal(err)
	}
	if decisions != 2 {
		t.Fatalf("audited recovery decisions = %d, want two once each", decisions)
	}
}

func newActivityHarness(t *testing.T) *activityHarness {
	t.Helper()
	pool := activityDatabase(t)
	begin := time.Date(2026, 8, 12, 23, 0, 0, 0, time.UTC)
	harness := &activityHarness{pool: pool, begin: begin, end: begin.Add(time.Hour), now: begin.Add(65 * time.Minute)}
	harness.activities = &Activities{Pool: pool, Events: storage.NewPostgresEventStore(pool), Now: func() time.Time { return harness.now }, MaxGap: 10 * time.Minute}
	ctx := context.Background()
	seeds := []struct {
		sql   string
		at    time.Time
		until time.Time
	}{
		{`INSERT INTO dispatch_requests (request_id, event_type, begin_time, end_time, target_kw, measurement_boundary, load_zones, correlation_id)
		 VALUES ('request-1', 'GRID_SERVICE', $1, $2, 7, 'METER_NET_EXPORT', ARRAY['LZ_AEN'], 'correlation-1')`, begin, harness.end},
		{`INSERT INTO dispatch_events (event_id, request_id, state, plan_version, correlation_id)
		 VALUES ('event-1', 'request-1', 'ACKNOWLEDGED_OR_UNCERTAIN', 1, 'correlation-1')`, time.Time{}, time.Time{}},
		{`INSERT INTO input_snapshots (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
		 VALUES ('input-1', 'event-1', $1, '{}', '{}', 'correlation-1')`, begin, time.Time{}},
		{`INSERT INTO eligibility_snapshots (snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
		 VALUES ('eligibility-1', 'event-1', $1, ARRAY['device-1', 'device-2'], '[]', 'policy-1', 'correlation-1')`, begin, time.Time{}},
		{`INSERT INTO plan_versions (event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
		 VALUES ('event-1', 1, 'input-1', 'eligibility-1', '{}', 'solver-1', 'model-1', 'correlation-1')`, time.Time{}, time.Time{}},
	}
	for _, seed := range seeds {
		arguments := make([]time.Time, 0, 2)
		for _, argument := range []time.Time{seed.at, seed.until} {
			if !argument.IsZero() {
				arguments = append(arguments, argument)
			}
		}
		var err error
		switch len(arguments) {
		case 0:
			_, err = pool.Exec(ctx, seed.sql)
		case 1:
			_, err = pool.Exec(ctx, seed.sql, arguments[0])
		default:
			_, err = pool.Exec(ctx, seed.sql, arguments[0], arguments[1])
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range []storage.CommandIntent{harness.command("command-1", "device-1", 4), harness.command("command-2", "device-2", 3)} {
		if err := storage.InsertCommand(ctx, pool, command); err != nil {
			t.Fatal(err)
		}
		if _, err := storage.TransitionCommand(ctx, pool, storage.CommandTransition{CommandID: command.CommandID, ExpectedState: "PERSISTED", NextState: "SENT", OccurredAt: begin, CorrelationID: "correlation-1"}); err != nil {
			t.Fatal(err)
		}
	}
	err := storage.RecordAcknowledgement(ctx, pool, storage.Acknowledgement{
		AcknowledgementID: "acknowledgement-1", CommandID: "command-1", IdempotencyKey: "acknowledgement-1", ReceiptStatus: "ACCEPTED",
		ReceivedAt: begin, GatewayID: "gateway-1", CorrelationID: "correlation-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	interval, err := FeasibleInterval(UncertainSend{
		DeviceID: "device-2", LastConfirmed: Command{ID: "command-0", SetpointKW: 0, EffectiveAt: begin.Add(-time.Hour), ExpiresAt: begin},
		PossiblyAccepted: Command{ID: "command-2", SetpointKW: 3, EffectiveAt: begin, ExpiresAt: harness.end},
		Fresh:            Telemetry{PowerKW: 0, ObservedAt: begin}, DerivedAt: begin.Add(time.Minute), CorrelationID: "correlation-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = storage.MarkAcknowledgementUncertain(ctx, pool, begin.Add(time.Minute), begin.Add(time.Minute), interval); err != nil {
		t.Fatal(err)
	}
	return harness
}

func (harness *activityHarness) command(id, deviceID string, setpointKW float64) storage.CommandIntent {
	return storage.CommandIntent{
		CommandID: id, IdempotencyKey: id, DeviceID: deviceID, EventID: "event-1", PlanVersion: 1, Generation: 1, SetpointKW: setpointKW,
		IssuedAt: harness.begin.Add(-time.Minute), EffectiveAt: harness.begin, ExpiresAt: harness.end, PolicyVersion: "policy-1", CorrelationID: "correlation-1",
	}
}

func (harness *activityHarness) observeExport(t *testing.T, minuteOffsets ...int) {
	t.Helper()
	observations := make([]*gridosv1.TelemetryObservation, 0, len(minuteOffsets))
	for _, offset := range minuteOffsets {
		at := harness.begin.Add(time.Duration(offset) * time.Minute)
		observations = append(observations, &gridosv1.TelemetryObservation{
			ObservationId: fmt.Sprintf("device-1-%d", offset), DeviceId: "device-1", Sequence: uint64(offset + 1000),
			SourceTime: timestamppb.New(at), ReceiveTime: timestamppb.New(harness.now), ObservationTime: timestamppb.New(at),
			ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT, MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT,
			PowerFlow: &gridosv1.PowerFlow{FromGridKw: -4, FromStorageKw: 4},
		})
	}
	if _, err := storage.NewTelemetryStoreAt(harness.pool, func() time.Time { return harness.now }).Write(context.Background(), "reconciliation-gateway", observations); err != nil {
		t.Fatal(err)
	}
}

func (harness *activityHarness) summary(t *testing.T, at time.Time) (float64, float64) {
	t.Helper()
	var deliveredKW, confidence float64
	err := harness.pool.QueryRow(context.Background(), `SELECT delivered_kw, confidence FROM verification_summaries
		WHERE event_id = 'event-1' AND interval_begin_time = $1`, at).Scan(&deliveredKW, &confidence)
	if err != nil {
		t.Fatal(err)
	}
	return deliveredKW, confidence
}

func (harness *activityHarness) eventState(t *testing.T) string {
	t.Helper()
	var state string
	if err := harness.pool.QueryRow(context.Background(), "SELECT state FROM dispatch_events WHERE event_id = 'event-1'").Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

func (harness *activityHarness) count(t *testing.T, table string) int {
	t.Helper()
	var rows int
	if err := harness.pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func activityDatabase(t *testing.T) *pgxpool.Pool {
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
	name := fmt.Sprintf("gridos_reconciliation_%d", time.Now().UnixNano())
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
	files, err := filepath.Glob(filepath.Join(filepath.Dir(currentFile), "../../../../database/migrations/*.sql"))
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

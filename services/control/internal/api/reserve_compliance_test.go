package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestMeasuredReserveComplianceFromTelemetry(t *testing.T) {
	pool := apiTestDatabase(t)
	seedAPIEvent(t, pool)
	ctx := context.Background()
	var begin time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT begin_time FROM dispatch_requests WHERE request_id = 'request-restart'`).Scan(&begin))
	frozen := &gridosv1.OptimizationRequest{EventId: "event-restart", PlanVersion: 3, Devices: []*gridosv1.DeviceState{
		{DeviceId: "device-1", UsableEnergyKwh: 10, EffectiveReserveKwh: 5},
		{DeviceId: "device-2", UsableEnergyKwh: 10, EffectiveReserveKwh: 3},
	}}
	inputs, err := protojson.Marshal(frozen)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE input_snapshots SET inputs = $1 WHERE snapshot_id = 'input-restart'`, inputs)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO telemetry_observations (observed_at, device_id, sequence, observation_id, payload)
		VALUES ($1::timestamptz + interval '1 minute', 'device-1', 1, 'reserve-1', '{"valueState":"VALUE_STATE_PRESENT","stateOfEnergyPercent":51}'),
		($1::timestamptz + interval '2 minutes', 'device-1', 2, 'reserve-2', '{"valueState":"VALUE_STATE_PRESENT","stateOfEnergyPercent":50}'),
		($1::timestamptz + interval '3 minutes', 'device-1', 3, 'reserve-3', '{"valueState":"VALUE_STATE_MISSING"}'),
		($1::timestamptz + interval '1 minute', 'device-2', 1, 'reserve-missing', '{"valueState":"VALUE_STATE_MISSING"}')`, begin)
	require.NoError(t, err)
	seedDischargeIntents(t, pool, begin, "device-1", "device-2")
	built, err := report.Build(ctx, NewPostgresReportSource(pool), "event-restart")
	require.NoError(t, err)
	require.Equal(t, "event-restart", built.EventID)
	encoded, err := json.Marshal(built)
	require.NoError(t, err)
	var observed struct {
		ReserveCompliance *struct {
			DevicesExpected     uint64   `json:"DevicesExpected"`
			DevicesObserved     uint64   `json:"DevicesObserved"`
			MinimumMarginKWh    *float64 `json:"MinimumMarginKWh"`
			DevicesTouchedFloor uint64   `json:"DevicesTouchedFloor"`
			ObservationGaps     uint64   `json:"ObservationGaps"`
			ValueKind           string   `json:"ValueKind"`
			Provenance          []string `json:"Provenance"`
		} `json:"ReserveCompliance"`
	}
	require.NoError(t, json.Unmarshal(encoded, &observed))
	if observed.ReserveCompliance == nil {
		t.Fatalf("measured reserve compliance missing from report: %s", encoded)
	}
	compliance := observed.ReserveCompliance
	require.Equal(t, uint64(2), compliance.DevicesExpected)
	require.Equal(t, uint64(1), compliance.DevicesObserved)
	require.NotNil(t, compliance.MinimumMarginKWh)
	require.InDelta(t, 0, *compliance.MinimumMarginKWh, 1e-9)
	require.Equal(t, uint64(1), compliance.DevicesTouchedFloor)
	require.Equal(t, uint64(2), compliance.ObservationGaps)
	require.Equal(t, "MEASURED", compliance.ValueKind)
	require.Contains(t, compliance.Provenance, "TELEMETRY_OBSERVATIONS")
	require.Contains(t, compliance.Provenance, "FROZEN_EFFECTIVE_RESERVE")
}

func TestReserveComplianceMeasuresOnlyDischargedDevices(t *testing.T) {
	pool := apiTestDatabase(t)
	seedAPIEvent(t, pool)
	ctx := context.Background()
	var begin time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT begin_time FROM dispatch_requests WHERE request_id = 'request-restart'`).Scan(&begin))
	frozen := &gridosv1.OptimizationRequest{EventId: "event-restart", PlanVersion: 3, Devices: []*gridosv1.DeviceState{
		{DeviceId: "device-dispatched", UsableEnergyKwh: 10, EffectiveReserveKwh: 5},
		{DeviceId: "device-idle-below-floor", UsableEnergyKwh: 10, EffectiveReserveKwh: 8},
	}}
	inputs, err := protojson.Marshal(frozen)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE input_snapshots SET inputs = $1 WHERE snapshot_id = 'input-restart'`, inputs)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO telemetry_observations (observed_at, device_id, sequence, observation_id, payload)
		VALUES ($1::timestamptz + interval '1 minute', 'device-dispatched', 1, 'dispatched-1', '{"valueState":"VALUE_STATE_PRESENT","stateOfEnergyPercent":52}'),
		($1::timestamptz + interval '1 minute', 'device-idle-below-floor', 1, 'idle-1', '{"valueState":"VALUE_STATE_PRESENT","stateOfEnergyPercent":50}')`, begin)
	require.NoError(t, err)
	seedDischargeIntents(t, pool, begin, "device-dispatched")
	_, err = pool.Exec(ctx, `INSERT INTO command_intents (command_id, idempotency_key, device_id, event_id, plan_version, generation, setpoint_kw, issued_at, effective_at, expires_at, policy_version, correlation_id)
		VALUES ('zero-idle', 'zero-idle', 'device-idle-below-floor', 'event-restart', 3, 1, 0, $1, $1, $1::timestamptz + interval '1 minute', 'policy-1', 'restart')`, begin)
	require.NoError(t, err)
	built, err := report.Build(ctx, NewPostgresReportSource(pool), "event-restart")
	require.NoError(t, err)
	require.NotNil(t, built.ReserveCompliance)
	require.Equal(t, uint64(1), built.ReserveCompliance.DevicesExpected)
	require.Equal(t, uint64(1), built.ReserveCompliance.DevicesObserved)
	require.InDelta(t, 0.2, *built.ReserveCompliance.MinimumMarginKWh, 1e-9)
	require.Equal(t, uint64(0), built.ReserveCompliance.DevicesTouchedFloor)
}

func seedDischargeIntents(t *testing.T, pool *pgxpool.Pool, begin time.Time, devices ...string) {
	t.Helper()
	for _, device := range devices {
		_, err := pool.Exec(context.Background(), `INSERT INTO command_intents (command_id, idempotency_key, device_id, event_id, plan_version, generation, setpoint_kw, issued_at, effective_at, expires_at, policy_version, correlation_id)
			VALUES ($1, $1, $2, 'event-restart', 3, 1, 1.5, $3, $3, $3::timestamptz + interval '10 minutes', 'policy-1', 'restart')`, "discharge-"+device, device, begin)
		require.NoError(t, err)
	}
}

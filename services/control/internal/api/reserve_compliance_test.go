package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
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
		($1::timestamptz + interval '1 minute', 'device-2', 1, 'reserve-missing', '{"valueState":"VALUE_STATE_MISSING"}')`, begin)
	require.NoError(t, err)
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
	require.Equal(t, uint64(1), compliance.ObservationGaps)
	require.Equal(t, "MEASURED", compliance.ValueKind)
	require.Contains(t, compliance.Provenance, "TELEMETRY_OBSERVATIONS")
	require.Contains(t, compliance.Provenance, "FROZEN_EFFECTIVE_RESERVE")
}

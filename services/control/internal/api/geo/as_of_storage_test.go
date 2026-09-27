package geo

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGeoAsOfReplaysRetainedTelemetry(t *testing.T) {
	ctx := context.Background()
	url := os.Getenv("GRIDOS_DATABASE_URL")
	if url == "" {
		url = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, url)
	require.NoError(t, err)
	name := fmt.Sprintf("gridos_geo_%d", time.Now().UnixNano())
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		require.NoError(t, admin.Close(ctx))
	})
	config, err := pgxpool.ParseConfig(url)
	require.NoError(t, err)
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `CREATE TABLE telemetry_observations(observed_at timestamptz NOT NULL,device_id text NOT NULL,sequence bigint NOT NULL,payload jsonb NOT NULL)`)
	require.NoError(t, err)
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	twin := fleet.NewTwin(30 * time.Second)
	sites, adapter, err := fleet.Load("../../../../../testdata/fleets/texas-50.jsonl", twin, now)
	require.NoError(t, err)
	device := sites[0].GetDevices()[0].GetDeviceId()
	for index, energy := range []float64{40, 80} {
		at := now.Add(time.Duration(index) * time.Minute)
		observation := &gridosv1.TelemetryObservation{DeviceId: device, ObservationId: fmt.Sprintf("obs-%d", index), ObservationTime: timestamppb.New(at), Sequence: uint64(index + 1), StateOfEnergyPercent: energy, ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT, OperatingState: &gridosv1.TelemetryObservation_OnGrid{OnGrid: &gridosv1.OnGrid{ObservedAt: timestamppb.New(at)}}}
		payload, err := protojson.Marshal(observation)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO telemetry_observations(observed_at,device_id,sequence,payload) VALUES ($1,$2,$3,$4)`, at, device, index+1, payload)
		require.NoError(t, err)
	}
	past, err := retainedSiteStates(ctx, pool, adapter, []string{device}, now.Add(10*time.Second))
	require.NoError(t, err)
	require.Len(t, past, 1)
	require.InDelta(t, 0.4*sites[0].GetDevices()[0].GetBatteryParameters().GetUsableEnergyKwh(), past[0].EnergyKWh, 0.0001)
	require.Equal(t, now, past[0].ObservedAt)
	newer, err := retainedSiteStates(ctx, pool, adapter, []string{device}, now.Add(time.Minute+10*time.Second))
	require.NoError(t, err)
	require.Len(t, newer, 1)
	require.Greater(t, newer[0].EnergyKWh, past[0].EnergyKWh)
	missing, err := retainedSiteStates(ctx, pool, adapter, []string{device}, now.Add(-time.Second))
	require.NoError(t, err)
	require.Empty(t, missing)
}

package storage

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestRetentionStoresTelemetryOutsideAuditJournal(t *testing.T) {
	pool := testDatabase(t)
	store := NewTelemetryStore(pool)
	now := time.Now().UTC().Truncate(time.Second)
	first := &gridosv1.TelemetryObservation{ObservationId: "retention-first", DeviceId: "device-retention", Sequence: 1, ObservationTime: timestamppb.New(now.Add(-time.Minute)), ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT}
	second := &gridosv1.TelemetryObservation{ObservationId: "retention-second", DeviceId: "device-retention", Sequence: 2, ObservationTime: timestamppb.New(now), ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT}
	inserted, err := store.Write(context.Background(), []*gridosv1.TelemetryObservation{first, second})
	require.NoError(t, err)
	require.Len(t, inserted, 2)
	inserted, err = store.Write(context.Background(), []*gridosv1.TelemetryObservation{second})
	require.NoError(t, err)
	require.Empty(t, inserted)
	var observations, auditRows int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM telemetry_observations WHERE device_id = 'device-retention'`).Scan(&observations))
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_journal WHERE action = 'TELEMETRY_RECEIVED'`).Scan(&auditRows))
	require.Equal(t, 2, observations)
	require.Zero(t, auditRows)
	latest, err := store.Latest(context.Background(), []string{"device-retention"}, now.Add(-30*time.Second))
	require.NoError(t, err)
	require.Len(t, latest, 1)
	require.Equal(t, uint64(2), latest[0].GetSequence())
	var partitioned bool
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_partitioned_table WHERE partrelid = 'telemetry_observations'::regclass)`).Scan(&partitioned))
	require.True(t, partitioned)
}

func TestRetentionRejectsExpiredObservationWithAudit(t *testing.T) {
	pool := testDatabase(t)
	store := NewTelemetryStore(pool)
	late := &gridosv1.TelemetryObservation{ObservationId: "retention-late", DeviceId: "device-retention", Sequence: 1, ObservationTime: timestamppb.New(time.Now().UTC().AddDate(0, 0, -8)), ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT}
	_, err := store.Write(context.Background(), []*gridosv1.TelemetryObservation{late})
	require.ErrorContains(t, err, "outside telemetry retention")
	var rejected, stored int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_journal WHERE action = 'TELEMETRY_REJECTED' AND resource_id = 'device-retention:1'`).Scan(&rejected))
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM telemetry_observations WHERE device_id = 'device-retention'`).Scan(&stored))
	require.Equal(t, 1, rejected)
	require.Zero(t, stored)
}

func TestRetentionPrunesOneExpiredPartition(t *testing.T) {
	pool := testDatabase(t)
	store := NewTelemetryStore(pool)
	_, err := pool.Exec(context.Background(), `CREATE TABLE telemetry_observations_old PARTITION OF telemetry_observations FOR VALUES FROM ('2000-01-01 00:00:00+00') TO ('2000-01-02 00:00:00+00')`)
	require.NoError(t, err)
	pruned, err := store.Prune(context.Background(), time.Now().UTC())
	require.NoError(t, err)
	require.True(t, pruned)
	var exists bool
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT to_regclass('telemetry_observations_old') IS NOT NULL`).Scan(&exists))
	require.False(t, exists)
}

package storage

import (
	"context"
	"os"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
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

func TestRiskBridgePersistsGatewayHeartbeatAndDeviceSourceAtomically(t *testing.T) {
	pool := testDatabase(t)
	now := time.Now().UTC().Truncate(time.Second)
	store := NewTelemetryStoreAt(pool, func() time.Time { return now })
	observation := &gridosv1.TelemetryObservation{ObservationId: "risk-observation-1", DeviceId: "risk-device-1",
		Sequence: 1, ObservationTime: timestamppb.New(now.Add(-time.Second)), ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT}
	inserted, err := store.Write(context.Background(), "gateway-1", []*gridosv1.TelemetryObservation{observation})
	require.NoError(t, err)
	require.Len(t, inserted, 1)
	var publishedAt, observedAt time.Time
	var count int
	var gatewayID string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT last_published_at,last_sequence_count FROM gateway_heartbeats WHERE gateway_id = 'gateway-1'`).Scan(&publishedAt, &count))
	require.Equal(t, now, publishedAt)
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT gateway_id,observed_at FROM gateway_device_sources WHERE device_id = 'risk-device-1'`).Scan(&gatewayID, &observedAt))
	require.Equal(t, "gateway-1", gatewayID)
	require.Equal(t, now.Add(-time.Second), observedAt)
	store = NewTelemetryStoreAt(pool, func() time.Time { return now.Add(10 * time.Second) })
	inserted, err = store.Write(context.Background(), "gateway-2", []*gridosv1.TelemetryObservation{observation})
	require.NoError(t, err)
	require.Empty(t, inserted)
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT gateway_id FROM gateway_device_sources WHERE device_id = 'risk-device-1'`).Scan(&gatewayID))
	require.Equal(t, "gateway-1", gatewayID)
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
	_, err := pool.Exec(context.Background(), `CREATE TABLE telemetry_observations_20000101 PARTITION OF telemetry_observations FOR VALUES FROM ('2000-01-01 00:00:00+00') TO ('2000-01-02 00:00:00+00')`)
	require.NoError(t, err)
	pruned, err := store.Prune(context.Background(), time.Now().UTC())
	require.NoError(t, err)
	require.True(t, pruned)
	var exists bool
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT to_regclass('telemetry_observations_20000101') IS NOT NULL`).Scan(&exists))
	require.False(t, exists)
}

func TestRetentionBackfillRemainsReadableOnDailyPartitionCreation(t *testing.T) {
	pool := testDatabase(t)
	store := NewTelemetryStore(pool)
	now := time.Now().UTC().Truncate(time.Second)
	observation := &gridosv1.TelemetryObservation{ObservationId: "legacy-retention", DeviceId: "legacy-device", Sequence: 1, ObservationTime: timestamppb.New(now), ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT}
	values, err := protojson.Marshal(observation)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `INSERT INTO audit_journal (occurred_at, actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1, 'legacy-device', 'TELEMETRY_RECEIVED', 'dispatch_event', 'legacy-device:1', $2, 'legacy-retention')`, now, values)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../../../database/migrations/0006_telemetry_retention.sql")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(migration))
	require.NoError(t, err)
	inserted, err := store.Write(context.Background(), []*gridosv1.TelemetryObservation{observation})
	require.NoError(t, err)
	require.Empty(t, inserted)
	latest, err := store.Latest(context.Background(), []string{"legacy-device"}, now.Add(-time.Minute))
	require.NoError(t, err)
	require.Len(t, latest, 1)
}

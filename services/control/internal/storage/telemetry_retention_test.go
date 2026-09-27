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
	latest, err := store.Latest(context.Background(), []string{"device-retention"})
	require.NoError(t, err)
	require.Len(t, latest, 1)
	require.Equal(t, uint64(2), latest[0].GetSequence())
	var partitioned bool
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_partitioned_table WHERE partrelid = 'telemetry_observations'::regclass)`).Scan(&partitioned))
	require.True(t, partitioned)
}

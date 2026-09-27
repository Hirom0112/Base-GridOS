package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTelemetryGatewayBatchesSerializeWithBoundedLocks(t *testing.T) {
	pool := testDatabase(t)
	ctx := context.Background()
	held, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = held.Rollback(ctx) })
	_, err = held.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "telemetry:gateway:lock-test")
	require.NoError(t, err)
	store := NewTelemetryStore(pool)
	now := time.Now().UTC().Truncate(time.Second)
	done := make(chan error, 2)
	for batch := range 2 {
		observations := make([]*gridosv1.TelemetryObservation, 0, 50)
		for index := range 50 {
			observations = append(observations, &gridosv1.TelemetryObservation{ObservationId: fmt.Sprintf("lock-%d-%d", batch, index), DeviceId: fmt.Sprintf("device-%d-%d", batch, index), Sequence: 1, ObservationTime: timestamppb.New(now), ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT})
		}
		go func() { _, err := store.Write(ctx, "lock-test", observations); done <- err }()
	}
	select {
	case err := <-done:
		t.Fatalf("gateway batch bypassed gateway lock: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	var locks int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND database = (SELECT oid FROM pg_database WHERE datname = current_database())`).Scan(&locks)
	require.NoError(t, err)
	require.LessOrEqual(t, locks, 3)
	require.NoError(t, held.Commit(ctx))
	for range 2 {
		require.NoError(t, <-done)
	}
	var stored int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM telemetry_observations WHERE device_id LIKE 'device-%'`).Scan(&stored))
	require.Equal(t, 100, stored)
}

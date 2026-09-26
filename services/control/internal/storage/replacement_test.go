package storage

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/stretchr/testify/require"
)

func TestStoreReplacementRetainsFrozenInputAndRecordsCurrentState(t *testing.T) {
	pool := testDatabase(t)
	store := NewPostgresEventStore(pool)
	now := time.Now().UTC()
	insertPlan(t, pool, "replacement-storage")
	_, err := pool.Exec(context.Background(), `UPDATE dispatch_events SET state = 'EXECUTING', plan_version = 1 WHERE event_id = 'replacement-storage'`)
	require.NoError(t, err)
	current := &gridosv1.OptimizationRequest{
		EventId: "replacement-storage", PlanVersion: 2, CorrelationId: "replacement-storage",
		EligibilitySnapshot: &gridosv1.EligibilitySnapshot{EventId: "replacement-storage", EligibleDeviceIds: []string{"device-2"}},
		ReservePolicy:       &gridosv1.ReservePolicy{PolicyVersion: "policy-1"},
		Devices:             []*gridosv1.DeviceState{{DeviceId: "device-2", EnergyKwh: 8}},
	}
	plan := &gridosv1.DispatchPlan{EventId: "replacement-storage", PlanVersion: 2, PlanId: "replacement-2", SolverVersion: "fallback", ModelVersion: "1"}

	require.NoError(t, store.StoreReplacement(context.Background(), "replacement-storage", 1, current, plan, "replace-2", now))
	var frozenID, replacementID, state string
	var version int64
	err = pool.QueryRow(context.Background(), `SELECT plan.input_snapshot_id, plan.replacement_snapshot_id, event.state, event.plan_version
		FROM plan_versions AS plan JOIN dispatch_events AS event ON event.event_id = plan.event_id
		WHERE plan.event_id = 'replacement-storage' AND plan.version = 2`).Scan(&frozenID, &replacementID, &state, &version)
	require.NoError(t, err)
	require.Equal(t, "input-replacement-storage", frozenID)
	require.NotEmpty(t, replacementID)
	require.Equal(t, "EXECUTING", state)
	require.Equal(t, int64(2), version)
	var auditCount int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_journal WHERE resource_id = 'replacement-storage' AND action = 'REPLACEMENT_PLANNED'`).Scan(&auditCount))
	require.Equal(t, 1, auditCount)
}

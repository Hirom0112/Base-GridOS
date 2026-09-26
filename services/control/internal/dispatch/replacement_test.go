package dispatch

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestReplacementStaysInsideApprovedEnvelope(t *testing.T) {
	harness := newActivityHarness(t)
	snapshotter := harness.activities.Dispatcher.Snapshots.(activitySnapshotter)
	snapshotter.snapshot.Optimization.EligibilitySnapshot.EligibleDeviceIds = []string{"device-1", "device-2"}
	for _, deviceID := range []string{"device-1", "device-2"} {
		snapshotter.snapshot.Optimization.Devices = append(snapshotter.snapshot.Optimization.Devices, &gridosv1.DeviceState{
			DeviceId: deviceID, UsableEnergyKwh: 10, EnergyKwh: 8, HardwareFloorKwh: 1, EffectiveReserveKwh: 2,
			MaxDischargeKw: 5, DischargeEfficiency: 1, AvailabilityProbability: 1,
			TelemetryObservedAt: timestamppb.New(harness.activities.Now().Add(-time.Second)),
		})
	}
	harness.persist(t)
	require.NoError(t, harness.activities.IssueReplacement(context.Background(), ReplacementCommand{EventID: harness.input.EventID, DeviceID: "device-1", Generation: 2}))
	var replacementDevice string
	require.NoError(t, harness.pool.QueryRow(context.Background(), `SELECT device_id FROM command_intents WHERE generation = 2`).Scan(&replacementDevice))
	require.Equal(t, "device-2", replacementDevice)
	require.NoError(t, harness.activities.IssueReplacement(context.Background(), ReplacementCommand{EventID: harness.input.EventID, DeviceID: "outside-envelope", Generation: 3}))
	var outsideCount int
	require.NoError(t, harness.pool.QueryRow(context.Background(), `SELECT count(*) FROM command_intents WHERE generation = 3`).Scan(&outsideCount))
	require.Zero(t, outsideCount)
}

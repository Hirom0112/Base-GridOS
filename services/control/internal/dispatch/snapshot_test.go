package dispatch

import (
	"context"
	"testing"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestSnapshotRetryRejectsChangedLiveInputs(t *testing.T) {
	harness := newActivityHarness(t)
	snapshotter := harness.activities.Dispatcher.Snapshots.(activitySnapshotter)
	snapshotter.snapshot.Optimization.Devices = []*gridosv1.DeviceState{{DeviceId: "device-1", EnergyKwh: 5}}
	first, err := harness.activities.FreezeInputs(context.Background(), harness.input)
	require.NoError(t, err)
	require.NotEmpty(t, first.SnapshotDigest)

	snapshotter.snapshot.Optimization.Devices[0].EnergyKwh++
	_, err = harness.activities.FreezeInputs(context.Background(), harness.input)
	require.ErrorContains(t, err, "frozen snapshot changed")

	stored, err := storage.NewPostgresEventStore(harness.pool).LoadFrozen(context.Background(), harness.input.EventID, first.InputSnapshotID, first.EligibilitySnapshotID)
	require.NoError(t, err)
	require.NotEqual(t, snapshotter.snapshot.Optimization.Devices[0].GetEnergyKwh(), stored.GetDevices()[0].GetEnergyKwh())
}

func TestSnapshotApprovalSurvivesLiveChange(t *testing.T) {
	harness := newActivityHarness(t)
	harness.approve(t)
	require.NotEmpty(t, harness.input.ApprovalDigest)
	snapshotter := harness.activities.Dispatcher.Snapshots.(activitySnapshotter)
	snapshotter.snapshot.Optimization.ReservePolicy.PolicyVersion = "later-policy"
	require.NoError(t, harness.activities.PersistIntents(context.Background(), PersistInput{Input: harness.input, Launch: harness.launch()}))
	require.Equal(t, 1, harness.count(t, "command_intents"))
}

func TestSnapshotApprovalRejectsChangedDigest(t *testing.T) {
	harness := newActivityHarness(t)
	harness.approve(t)
	harness.input.ApprovalDigest[0] ^= 1
	err := harness.activities.PersistIntents(context.Background(), PersistInput{Input: harness.input, Launch: harness.launch()})
	require.ErrorContains(t, err, "approval digest changed")
	require.Zero(t, harness.count(t, "command_intents"))
}

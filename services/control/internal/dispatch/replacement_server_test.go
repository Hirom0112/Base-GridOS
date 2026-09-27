package dispatch

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	controlapi "github.com/Hirom0112/Base-GridOS/services/control/internal/api"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestReplacementUsesRealDecisionServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().(*net.TCPAddr)
	require.NoError(t, listener.Close())
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
	command := exec.Command("uv", "run", "--project", "services/decision", "python", "-m", "gridos.server", "--port", strconv.Itoa(address.Port))
	command.Dir = root
	command.Env = os.Environ()
	require.NoError(t, command.Start())
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	})
	deadline := time.Now().Add(10 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		connection, dialErr := net.DialTimeout("tcp", address.String(), 100*time.Millisecond)
		if dialErr == nil {
			_ = connection.Close()
			ready = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	require.True(t, ready, "decision server did not listen")
	transport := http.DefaultTransport.(*http.Transport).Clone()
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	transport.Protocols = protocols
	client := gridosv1connect.NewOptimizationServiceClient(&http.Client{Transport: transport, Timeout: 5 * time.Second}, "http://"+address.String(), connect.WithGRPC())
	optimizer := controlapi.NewConnectOptimizer(client)
	now := time.Now().UTC()
	begin := timestamppb.New(now.Add(time.Minute))
	end := timestamppb.New(now.Add(time.Hour))
	device := &gridosv1.DeviceState{DeviceId: "device-a", UsableEnergyKwh: 10, EnergyKwh: 8, HardwareFloorKwh: 1, EffectiveReserveKwh: 2, MaxDischargeKw: 5, DischargeEfficiency: 1, AvailabilityProbability: 1}
	request := &gridosv1.OptimizationRequest{
		RequestId: "replace-real", EventId: "replace-real", PlanVersion: 1, RequestedAt: timestamppb.New(now), Budget: durationpb.New(time.Second),
		MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT,
		Intervals:           []*gridosv1.OptimizationInterval{{BeginTime: begin, EndTime: end, TargetKw: 3}},
		Devices:             []*gridosv1.DeviceState{device},
	}
	approved, err := optimizer.Optimize(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, approved.GetDeviceSchedules(), 1)
	require.Equal(t, "device-a", approved.GetDeviceSchedules()[0].GetDeviceId())
	current := proto.Clone(request).(*gridosv1.OptimizationRequest)
	current.PlanVersion = 2
	current.Devices = append(current.Devices, proto.Clone(device).(*gridosv1.DeviceState), proto.Clone(device).(*gridosv1.DeviceState))
	current.Devices[1].DeviceId = "device-b"
	current.Devices[2].DeviceId = "outside-envelope"
	response, err := optimizer.Replace(context.Background(), &gridosv1.ReplaceRequest{
		Current: current, ApprovedPlan: approved, DroppedDeviceIds: []string{"device-a"},
		EnvelopeDeviceIds: []string{"device-a", "device-b"}, IdempotencyKey: "replace-real-2",
	})
	require.NoError(t, err)
	require.Len(t, response.GetReplacementPlan().GetDeviceSchedules(), 1)
	require.Equal(t, "device-b", response.GetReplacementPlan().GetDeviceSchedules()[0].GetDeviceId())
	require.Equal(t, 3.0, response.GetReplacementPlan().GetDeviceSchedules()[0].GetIntervals()[0].GetSetpointKw())
}

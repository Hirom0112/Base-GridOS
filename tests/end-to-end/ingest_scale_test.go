package endtoend

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
)

func TestIngestScale(t *testing.T) {
	requireDemo(t)
	client := gridosv1connect.NewFleetServiceClient(http.DefaultClient, controlURL)
	deadline := time.Now().Add(30 * time.Second)
	var online uint64
	var freshness time.Duration
	for time.Now().Before(deadline) {
		request := connect.NewRequest(&gridosv1.GetFleetSummaryRequest{LoadZones: []string{"LZ_AEN"}})
		request.Header().Set("X-GridOS-Role", "operator")
		response, err := client.GetFleetSummary(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		online = onlineDevices(response.Msg.GetSummary())
		freshness = response.Msg.GetSummary().GetDispatchableNowMw().GetMetadata().GetFreshness().AsDuration()
		if online == 5000 && freshness < 5*time.Second {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("online=%d freshness=%s", online, freshness)
}

func onlineDevices(summary *gridosv1.FleetSummary) uint64 {
	for _, count := range summary.GetAvailabilityStateCounts() {
		if count.GetAvailabilityState() == gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_ONLINE {
			return count.GetAggregate().GetDeviceCount()
		}
	}
	return 0
}

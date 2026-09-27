package geo

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestGeoServicePrivacyAndDrilldown(t *testing.T) {
	now := time.Unix(100, 0)
	sites := make([]*gridosv1.AuthorizedSite, 0, 6)
	states := make([]fleet.SiteState, 0, 6)
	for index := range 6 {
		id := fmt.Sprintf("site-%d", index)
		sites = append(sites, &gridosv1.AuthorizedSite{Site: &gridosv1.Site{SiteId: id, LoadZone: "LZ_AEN", H3Cell: "8726cb9a5ffffff", Provenance: &gridosv1.Provenance{Provenance: gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED}}, Devices: []*gridosv1.Device{{BatteryParameters: &gridosv1.BatteryParameters{UsableEnergyKwh: 10, MaxDischargeKw: 5}}}})
		states = append(states, fleet.SiteState{SiteID: id, EnergyKWh: 8, Availability: fleet.Online, OperatingState: fleet.OnGrid, ObservedAt: now, DispatchableKW: 5, Provenance: "simulated"})
	}
	sites[0].Devices = append(sites[0].Devices, &gridosv1.Device{BatteryParameters: &gridosv1.BatteryParameters{UsableEnergyKwh: 10, MaxDischargeKw: 5}})
	service := NewService(sites, func(context.Context, time.Time) ([]fleet.SiteState, map[string]bool, error) {
		return states, map[string]bool{"site-0": true}, nil
	}, func() time.Time { return now })
	_, handler := gridosv1connect.NewGeoServiceHandler(service)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	client := gridosv1connect.NewGeoServiceClient(httpServer.Client(), httpServer.URL)
	cellsRequest := connect.NewRequest(&gridosv1.ListCellsRequest{Resolution: 7})
	cellsRequest.Header().Set("X-GridOS-Role", "operator")
	cells, err := client.ListCells(context.Background(), cellsRequest)
	require.NoError(t, err)
	require.Len(t, cells.Msg.GetCells(), 1)
	require.Equal(t, uint64(6), cells.Msg.GetCells()[0].GetSiteCount())
	require.Equal(t, uint64(1), cells.Msg.GetCells()[0].GetActiveDispatchCount())
	require.Equal(t, gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED, cells.Msg.GetCells()[0].GetProvenance())
	require.Len(t, cells.Msg.GetCells()[0].GetOperatingStateCounts(), 1)
	require.Equal(t, uint64(7), cells.Msg.GetCells()[0].GetOperatingStateCounts()[0].GetAggregate().GetDeviceCount())
	parent := ""
	for range 5 {
		request := connect.NewRequest(&gridosv1.DrilldownRequest{ParentId: parent})
		request.Header().Set("X-GridOS-Role", "operator")
		response, err := client.Drilldown(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if len(response.Msg.GetNodes()) != 1 || response.Msg.GetNodes()[0].GetSiteCount() != 6 || response.Msg.GetNodes()[0].GetProvenance() != gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED {
			t.Fatalf("unsafe hierarchy: %+v", response.Msg.GetNodes())
		}
		parent = response.Msg.GetNodes()[0].GetId()
	}
	denied := connect.NewRequest(&gridosv1.DrilldownRequest{ParentId: parent})
	denied.Header().Set("X-GridOS-Role", "operator")
	if _, err := client.Drilldown(context.Background(), denied); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("site drilldown code = %v, want permission denied", connect.CodeOf(err))
	}
	allowed := connect.NewRequest(&gridosv1.DrilldownRequest{ParentId: parent})
	allowed.Header().Set("X-GridOS-Role", "operator")
	allowed.Header().Set("X-GridOS-Permissions", "site_location")
	exact, err := client.Drilldown(context.Background(), allowed)
	if err != nil || len(exact.Msg.GetSites()) != 6 {
		t.Fatalf("authorized site drilldown: %v, %+v", err, exact)
	}
	for _, response := range []proto.Message{cells.Msg, exact.Msg} {
		encoded, err := protojson.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateResponse(encoded); err != nil {
			t.Fatalf("private field in geo response: %v", err)
		}
	}
}

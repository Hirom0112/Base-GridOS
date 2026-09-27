package geo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGeoAsOfUsesRequestedSnapshot(t *testing.T) {
	current := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	past := current.Add(-time.Hour)
	sites := make([]*gridosv1.AuthorizedSite, 0, 6)
	for index := range 6 {
		sites = append(sites, &gridosv1.AuthorizedSite{Site: &gridosv1.Site{SiteId: fmt.Sprintf("site-%d", index), LoadZone: "LZ_AEN", H3Cell: "8726cb9a5ffffff", Provenance: &gridosv1.Provenance{Provenance: gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED}}})
	}
	var seen []time.Time
	var sources []SnapshotSource
	snapshot := func(_ context.Context, at time.Time, source SnapshotSource) ([]fleet.SiteState, map[string]bool, error) {
		seen = append(seen, at)
		sources = append(sources, source)
		states := make([]fleet.SiteState, 0, 6)
		for index := range 6 {
			states = append(states, fleet.SiteState{SiteID: fmt.Sprintf("site-%d", index), ObservedAt: at, OperatingState: fleet.OnGrid, Availability: fleet.Online, EnergyKWh: float64(at.Hour()), Provenance: "simulated"})
		}
		return states, nil, nil
	}
	service := NewService(sites, snapshot, func() time.Time { return current })
	cells := new(gridosv1.ListCellsRequest)
	require.NoError(t, protojson.Unmarshal([]byte(`{"resolution":7,"asOf":"2026-09-27T11:00:00Z"}`), cells))
	list := connect.NewRequest(cells)
	list.Header().Set("X-GridOS-Role", "operator")
	result, err := service.ListCells(context.Background(), list)
	require.NoError(t, err)
	require.Equal(t, past, result.Msg.GetAsOf().AsTime())
	drill := new(gridosv1.DrilldownRequest)
	require.NoError(t, protojson.Unmarshal([]byte(`{"asOf":"2026-09-27T11:00:00Z"}`), drill))
	request := connect.NewRequest(drill)
	request.Header().Set("X-GridOS-Role", "operator")
	nodes, err := service.Drilldown(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, past, nodes.Msg.GetAsOf().AsTime())
	require.Equal(t, []time.Time{past, past}, seen)
	require.Equal(t, []SnapshotSource{RetainedSnapshot, RetainedSnapshot}, sources)
	list.Msg.AsOf = nil
	currentCells, err := service.ListCells(context.Background(), list)
	require.NoError(t, err)
	require.Equal(t, current, currentCells.Msg.GetAsOf().AsTime())
	require.Equal(t, CurrentSnapshot, sources[2])
	list.Msg.AsOf = timestamppb.New(current.Add(time.Second))
	_, err = service.ListCells(context.Background(), list)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

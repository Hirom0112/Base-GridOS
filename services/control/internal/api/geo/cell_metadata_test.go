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
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCellMetadataCarriesAsOfFreshnessAndProvenanceMix(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	sites := make([]*gridosv1.AuthorizedSite, 0, 6)
	states := make([]fleet.SiteState, 0, 6)
	for index := range 6 {
		provenance := gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED
		name := "simulated"
		if index >= 3 {
			provenance = gridosv1.DataProvenance_DATA_PROVENANCE_DERIVED
			name = "derived"
		}
		id := fmt.Sprintf("site-%d", index)
		sites = append(sites, &gridosv1.AuthorizedSite{Site: &gridosv1.Site{SiteId: id,
			LoadZone: "LZ_AEN", H3Cell: "8726cb9a5ffffff", Provenance: &gridosv1.Provenance{Provenance: provenance}}})
		states = append(states, fleet.SiteState{SiteID: id, ObservedAt: now.Add(-time.Duration(index+1) * time.Second),
			Provenance: name, OperatingState: fleet.OnGrid, Availability: fleet.Online})
	}
	service := NewService(sites, func(context.Context, time.Time, SnapshotSource) ([]fleet.SiteState, map[string]bool, error) {
		return states, nil, nil
	}, func() time.Time { return now })
	list := connect.NewRequest(&gridosv1.ListCellsRequest{Resolution: 7})
	list.Header().Set("X-GridOS-Role", "operator")
	cells, err := service.ListCells(context.Background(), list)
	require.NoError(t, err)
	require.Len(t, cells.Msg.GetCells(), 1)
	assertGeoMetadata(t, cells.Msg.GetCells()[0].ProtoReflect(), now)
	assertGeoMetadata(t, cells.Msg.ProtoReflect(), now)
	drill := connect.NewRequest(&gridosv1.DrilldownRequest{})
	drill.Header().Set("X-GridOS-Role", "operator")
	nodes, err := service.Drilldown(context.Background(), drill)
	require.NoError(t, err)
	require.Len(t, nodes.Msg.GetNodes(), 1)
	assertGeoMetadata(t, nodes.Msg.GetNodes()[0].ProtoReflect(), now)
	assertGeoMetadata(t, nodes.Msg.ProtoReflect(), now)
}

func assertGeoMetadata(t *testing.T, value protoreflect.Message, at time.Time) {
	t.Helper()
	fields := value.Descriptor().Fields()
	asOf := fields.ByName("as_of")
	metadata := fields.ByName("metadata")
	require.NotNil(t, asOf)
	require.NotNil(t, metadata)
	require.Equal(t, at, value.Get(asOf).Message().Interface().(*timestamppb.Timestamp).AsTime())
	require.Equal(t, 6*time.Second, value.Get(fields.ByName("freshness")).Message().Interface().(*durationpb.Duration).AsDuration())
	aggregate := value.Get(metadata).Message().Interface().(*gridosv1.AggregateMetadata)
	require.Equal(t, at, aggregate.GetTimestamp().AsTime())
	require.Equal(t, 6*time.Second, aggregate.GetFreshness().AsDuration())
	counts := make(map[gridosv1.DataProvenance]uint64)
	for _, share := range aggregate.GetProvenanceMix() {
		counts[share.GetProvenance()] = share.GetRecordCount()
	}
	require.Equal(t, uint64(3), counts[gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED])
	require.Equal(t, uint64(3), counts[gridosv1.DataProvenance_DATA_PROVENANCE_DERIVED])
}

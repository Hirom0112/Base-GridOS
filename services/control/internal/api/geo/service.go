package geo

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	fleetgeo "github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/geo"
	"github.com/jackc/pgx/v5/pgxpool"
	h3 "github.com/uber/h3-go/v4"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Snapshot func(context.Context, time.Time) ([]fleet.SiteState, map[string]bool, error)

type Service struct {
	sites    []*gridosv1.AuthorizedSite
	snapshot Snapshot
	now      func() time.Time
}

func NewService(sites []*gridosv1.AuthorizedSite, snapshot Snapshot, now func() time.Time) *Service {
	return &Service{sites: sites, snapshot: snapshot, now: now}
}

func PostgresSnapshot(sites []*gridosv1.AuthorizedSite, twin *fleet.Twin, pool *pgxpool.Pool) Snapshot {
	siteByDevice := make(map[string]string)
	for _, site := range sites {
		for _, device := range site.GetDevices() {
			siteByDevice[device.GetDeviceId()] = site.GetSite().GetSiteId()
		}
	}
	return func(ctx context.Context, now time.Time) ([]fleet.SiteState, map[string]bool, error) {
		rows, err := pool.Query(ctx, `SELECT DISTINCT intent.device_id FROM command_intents AS intent
			JOIN LATERAL (SELECT state FROM command_states WHERE command_id = intent.command_id
				ORDER BY recorded_at DESC LIMIT 1) AS latest ON true
			WHERE intent.effective_at <= $1 AND intent.expires_at > $1 AND intent.setpoint_kw <> 0
			AND latest.state IN ('ACKNOWLEDGED', 'EXECUTING')`, now)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()
		active := make(map[string]bool)
		for rows.Next() {
			var deviceID string
			if err := rows.Scan(&deviceID); err != nil {
				return nil, nil, err
			}
			if siteID := siteByDevice[deviceID]; siteID != "" {
				active[siteID] = true
			}
		}
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}
		return twin.Sites(now), active, nil
	}
}

func (service *Service) ListCells(ctx context.Context, request *connect.Request[gridosv1.ListCellsRequest]) (*connect.Response[gridosv1.ListCellsResponse], error) {
	if err := authorize(request.Header()); err != nil {
		return nil, err
	}
	resolution := int(request.Msg.GetResolution())
	if resolution < 5 || resolution > 7 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("resolution must be 5 through 7"))
	}
	selected := selectSites(service.sites, request.Msg.GetLoadZones())
	now := service.now()
	states, active, err := service.snapshot(ctx, now)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	cells, err := fleetgeo.AggregatePrivate(selected, states, active, now, resolution)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &gridosv1.ListCellsResponse{Cells: make([]*gridosv1.GeoCell, 0, len(cells))}
	indexed := statesBySite(states)
	response.Metadata = metadataForSites(selected, indexed, now)
	response.AsOf = response.Metadata.GetTimestamp()
	response.Freshness = response.Metadata.GetFreshness()
	counts, err := countDevicesByCell(selected, states, cells)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, cell := range cells {
		encoded, err := encodeCell(cell, counts[cell.Cell], indexed, now)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		response.Cells = append(response.Cells, encoded)
	}
	return connect.NewResponse(response), nil
}

type deviceStateCounts struct {
	operating    map[fleet.OperatingState]uint64
	availability map[fleet.Availability]uint64
	sites        []*gridosv1.AuthorizedSite
}

func countDevicesByCell(sites []*gridosv1.AuthorizedSite, states []fleet.SiteState, cells []fleetgeo.Cell) (map[string]*deviceStateCounts, error) {
	output := make(map[string]*deviceStateCounts, len(cells))
	for _, cell := range cells {
		output[cell.Cell] = &deviceStateCounts{operating: make(map[fleet.OperatingState]uint64), availability: make(map[fleet.Availability]uint64)}
	}
	bySite := make(map[string]fleet.SiteState, len(states))
	for _, state := range states {
		bySite[state.SiteID] = state
	}
	for _, site := range sites {
		state, found := bySite[site.GetSite().GetSiteId()]
		cell := h3.CellFromString(site.GetSite().GetH3Cell())
		for resolution := cell.Resolution(); resolution >= 0; resolution-- {
			parent, err := cell.Parent(resolution)
			if err != nil {
				return nil, err
			}
			if counts := output[parent.String()]; counts != nil {
				counts.sites = append(counts.sites, site)
				if !found {
					break
				}
				devices := uint64(len(site.GetDevices()))
				counts.operating[state.OperatingState] += devices
				counts.availability[state.Availability] += devices
				break
			}
		}
	}
	return output, nil
}

func (service *Service) Drilldown(ctx context.Context, request *connect.Request[gridosv1.DrilldownRequest]) (*connect.Response[gridosv1.DrilldownResponse], error) {
	if err := authorize(request.Header()); err != nil {
		return nil, err
	}
	if len(service.sites) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no authorized sites for drilldown"))
	}
	now := service.now()
	states, _, err := service.snapshot(ctx, now)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response, err := drilldown(service.sites, request.Msg.GetParentId(), hasSiteLocation(request.Header()), statesBySite(states), now)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(response), nil
}

func selectSites(sites []*gridosv1.AuthorizedSite, zones []string) []*gridosv1.AuthorizedSite {
	if len(zones) == 0 {
		return sites
	}
	selected := make([]*gridosv1.AuthorizedSite, 0)
	for _, site := range sites {
		for _, zone := range zones {
			if site.GetSite().GetLoadZone() == zone {
				selected = append(selected, site)
				break
			}
		}
	}
	return selected
}

func encodeCell(cell fleetgeo.Cell, counts *deviceStateCounts, states map[string]fleet.SiteState, now time.Time) (*gridosv1.GeoCell, error) {
	metadata := metadataForSites(counts.sites, states, now)
	encoded := &gridosv1.GeoCell{
		H3Cell: cell.Cell, SiteCount: cell.SiteCount, InstalledMw: cell.InstalledMW,
		InstalledMwh: cell.InstalledMWh, DispatchableMw: cell.DispatchableMW, ReservedMwh: cell.ReservedMWh,
		SocLowCount: cell.SOCLowCount, SocMediumCount: cell.SOCMediumCount, SocHighCount: cell.SOCHighCount,
		SocUnknownCount: cell.SOCUnknownCount, ConnectedCount: cell.ConnectedCount,
		ActiveDispatchCount: cell.ActiveDispatchCount, Freshness: metadata.GetFreshness(),
		Provenance: singleProvenance(metadata), AsOf: metadata.GetTimestamp(), Metadata: metadata,
	}
	countMetadata := func(count uint64) *gridosv1.AggregateMetadata {
		return &gridosv1.AggregateMetadata{
			Timestamp: timestamppb.New(now), Freshness: durationpb.New(cell.Freshness),
			ProvenanceMix: []*gridosv1.ProvenanceShare{{Provenance: gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED, RecordCount: count}},
		}
	}
	for state, count := range counts.operating {
		value, found := gridosv1.FleetOperatingState_value["FLEET_OPERATING_STATE_"+string(state)]
		if !found {
			return nil, errors.New("unknown fleet operating state")
		}
		encoded.OperatingStateCounts = append(encoded.OperatingStateCounts, &gridosv1.OperatingStateDeviceCount{OperatingState: gridosv1.FleetOperatingState(value), Aggregate: &gridosv1.FleetDeviceCountAggregate{DeviceCount: count, Metadata: countMetadata(count)}})
	}
	for state, count := range counts.availability {
		value, found := gridosv1.FleetAvailabilityState_value["FLEET_AVAILABILITY_STATE_"+string(state)]
		if !found {
			return nil, errors.New("unknown fleet availability state")
		}
		encoded.AvailabilityStateCounts = append(encoded.AvailabilityStateCounts, &gridosv1.AvailabilityStateDeviceCount{AvailabilityState: gridosv1.FleetAvailabilityState(value), Aggregate: &gridosv1.FleetDeviceCountAggregate{DeviceCount: count, Metadata: countMetadata(count)}})
	}
	sort.Slice(encoded.OperatingStateCounts, func(i, j int) bool {
		return encoded.OperatingStateCounts[i].OperatingState < encoded.OperatingStateCounts[j].OperatingState
	})
	sort.Slice(encoded.AvailabilityStateCounts, func(i, j int) bool {
		return encoded.AvailabilityStateCounts[i].AvailabilityState < encoded.AvailabilityStateCounts[j].AvailabilityState
	})
	return encoded, nil
}

func authorize(header http.Header) error {
	role := header.Get("X-GridOS-Role")
	if role == "" {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("role required"))
	}
	for _, allowed := range []string{"operator", "approver", "analyst", "partner", "service"} {
		if role == allowed {
			return nil
		}
	}
	return connect.NewError(connect.CodePermissionDenied, errors.New("role is not authorized"))
}

func hasSiteLocation(header http.Header) bool {
	for _, permission := range strings.FieldsFunc(header.Get("X-GridOS-Permissions"), func(r rune) bool { return r == ',' || r == ' ' }) {
		if permission == "site_location" {
			return true
		}
	}
	return false
}

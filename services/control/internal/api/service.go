package api

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	roleHeader        = "X-GridOS-Role"
	permissionsHeader = "X-GridOS-Permissions"
	siteLocation      = "site_location"
)

type Service struct {
	store EventStore
	twin  *fleet.Twin
	sites []*gridosv1.AuthorizedSite
	now   func() time.Time
}

func NewService(store EventStore, twin *fleet.Twin, sites []*gridosv1.AuthorizedSite, now func() time.Time) *Service {
	return &Service{store: store, twin: twin, sites: sites, now: now}
}

func NewHandler(service *Service) http.Handler {
	mux := http.NewServeMux()
	fleetPath, fleetHandler := gridosv1connect.NewFleetServiceHandler(service)
	dispatchPath, dispatchHandler := gridosv1connect.NewDispatchServiceHandler(service)
	mux.Handle(fleetPath, fleetHandler)
	mux.Handle(dispatchPath, dispatchHandler)
	return mux
}

func (service *Service) GetFleetSummary(_ context.Context, request *connect.Request[gridosv1.GetFleetSummaryRequest]) (*connect.Response[gridosv1.GetFleetSummaryResponse], error) {
	if err := authorize(request.Header(), "operator", "approver", "analyst", "partner", "service"); err != nil {
		return nil, err
	}
	now := service.now()
	aggregate := service.twin.Aggregate(now)
	states := service.twin.Sites(now)
	installedMW, installedMWh := installedCapacity(service.sites)
	summary := &gridosv1.FleetSummary{
		InstalledMw:                 quantity(installedMW, now, 0, nil),
		InstalledMwh:                quantity(installedMWh, now, 0, nil),
		DispatchableNowMw:           quantity(aggregate.DispatchableMW.Value, now, aggregate.DispatchableMW.Freshness, aggregate.DispatchableMW.ProvenanceMix),
		ForecastDispatchableMw:      quantity(aggregate.DispatchableMW.Value, now, aggregate.DispatchableMW.Freshness, aggregate.DispatchableMW.ProvenanceMix),
		ReservedForBackupMwh:        quantity(max(installedMWh-aggregate.DispatchableMWh.Value, 0), now, aggregate.DispatchableMWh.Freshness, aggregate.DispatchableMWh.ProvenanceMix),
		OperatingStateCounts:        operatingCounts(states, now),
		AvailabilityStateCounts:     availabilityCounts(states, now),
		CommunicationsHealthCounts:  healthCounts(states, now),
		AcknowledgementHealthCounts: healthCounts(states, now),
	}
	return connect.NewResponse(&gridosv1.GetFleetSummaryResponse{Summary: summary}), nil
}

func (service *Service) ListSites(_ context.Context, request *connect.Request[gridosv1.ListSitesRequest]) (*connect.Response[gridosv1.ListSitesResponse], error) {
	if err := authorize(request.Header(), "operator", "approver", "analyst", "partner", "service"); err != nil {
		return nil, err
	}
	selected := filterSites(service.sites, request.Msg.GetLoadZones())
	if request.Msg.GetRequestExactH3Cells() {
		if !hasPermission(request.Header(), siteLocation) {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("site_location permission required"))
		}
		locations := make([]*gridosv1.SiteLocation, 0, len(selected))
		for _, site := range selected {
			locations = append(locations, &gridosv1.SiteLocation{Location: &gridosv1.SiteLocation_Exact{Exact: site}})
		}
		return connect.NewResponse(&gridosv1.ListSitesResponse{Sites: locations}), nil
	}
	return connect.NewResponse(&gridosv1.ListSitesResponse{Sites: aggregateSites(selected, service.now())}), nil
}

func (service *Service) CreateEventRequest(ctx context.Context, request *connect.Request[gridosv1.CreateEventRequestRequest]) (*connect.Response[gridosv1.CreateEventRequestResponse], error) {
	if err := authorize(request.Header(), "operator"); err != nil {
		return nil, err
	}
	eventRequest := request.Msg.GetEventRequest()
	if eventRequest == nil || eventRequest.GetRequestId() == "" || eventRequest.GetBeginTime() == nil || eventRequest.GetEndTime() == nil || !eventRequest.GetEndTime().AsTime().After(eventRequest.GetBeginTime().AsTime()) || eventRequest.GetTargetKw() < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid event request required"))
	}
	event, err := service.store.Create(ctx, eventRequest, request.Msg.GetIdempotencyKey(), service.now())
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&gridosv1.CreateEventRequestResponse{Event: event}), nil
}

func (service *Service) GetEvent(ctx context.Context, request *connect.Request[gridosv1.GetEventRequest]) (*connect.Response[gridosv1.GetEventResponse], error) {
	if err := authorize(request.Header(), "operator", "approver", "analyst", "partner", "service"); err != nil {
		return nil, err
	}
	if request.Msg.GetEventId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event_id required"))
	}
	event, exclusions, err := service.store.Get(ctx, request.Msg.GetEventId())
	if err != nil {
		return nil, storeError(err)
	}
	groups := make([]*gridosv1.ExclusionReasonGroup, 0, len(exclusions))
	for reason, count := range exclusions {
		groups = append(groups, &gridosv1.ExclusionReasonGroup{Reason: reason, Count: count})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].GetReason() < groups[j].GetReason() })
	return connect.NewResponse(&gridosv1.GetEventResponse{Event: event, Exclusions: groups}), nil
}

func (service *Service) ApproveEvent(ctx context.Context, request *connect.Request[gridosv1.ApproveEventRequest]) (*connect.Response[gridosv1.ApproveEventResponse], error) {
	if err := authorize(request.Header(), "approver"); err != nil {
		return nil, err
	}
	if request.Msg.GetEventId() == "" || request.Msg.GetPlanVersion() == 0 || request.Msg.GetApprovedBy() == "" || request.Msg.GetApprovedAt() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("complete approval required"))
	}
	event, err := service.store.Approve(ctx, request.Msg)
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&gridosv1.ApproveEventResponse{Event: event}), nil
}

func (service *Service) LaunchEvent(ctx context.Context, request *connect.Request[gridosv1.LaunchEventRequest]) (*connect.Response[gridosv1.LaunchEventResponse], error) {
	if err := authorize(request.Header(), "approver"); err != nil {
		return nil, err
	}
	if request.Msg.GetEventId() == "" || request.Msg.GetPlanVersion() == 0 || request.Msg.GetRequestedBy() == "" || request.Msg.GetRequestedAt() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("complete launch request required"))
	}
	event, err := service.store.Launch(ctx, request.Msg)
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&gridosv1.LaunchEventResponse{Event: event}), nil
}

func authorize(header http.Header, roles ...string) error {
	role := header.Get(roleHeader)
	if role == "" {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("role required"))
	}
	for _, allowed := range roles {
		if role == allowed {
			return nil
		}
	}
	return connect.NewError(connect.CodePermissionDenied, errors.New("role is not authorized"))
}

func hasPermission(header http.Header, permission string) bool {
	for _, candidate := range strings.FieldsFunc(header.Get(permissionsHeader), func(r rune) bool { return r == ',' || r == ' ' }) {
		if candidate == permission {
			return true
		}
	}
	return false
}

func storeError(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, ErrInvalidState), errors.Is(err, ErrPlanVersion):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, ErrIdempotencyKey):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func quantity(value float64, now time.Time, freshness time.Duration, provenance map[string]int) *gridosv1.FleetQuantityAggregate {
	shares := make([]*gridosv1.ProvenanceShare, 0, len(provenance))
	for source, count := range provenance {
		shares = append(shares, &gridosv1.ProvenanceShare{Provenance: provenanceValue(source), RecordCount: uint64(count)})
	}
	sort.Slice(shares, func(i, j int) bool { return shares[i].GetProvenance() < shares[j].GetProvenance() })
	return &gridosv1.FleetQuantityAggregate{Value: value, Metadata: &gridosv1.AggregateMetadata{Timestamp: timestamp(now), ProvenanceMix: shares, Freshness: durationpb.New(freshness)}}
}

func provenanceValue(source string) gridosv1.DataProvenance {
	values := map[string]gridosv1.DataProvenance{
		"confirmed_public":       gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC,
		"confirmed_sandbox":      gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_SANDBOX,
		"authorized_operational": gridosv1.DataProvenance_DATA_PROVENANCE_AUTHORIZED_OPERATIONAL,
		"derived":                gridosv1.DataProvenance_DATA_PROVENANCE_DERIVED,
		"simulated":              gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED,
	}
	return values[source]
}

package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	reporting "github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	roleHeader        = "X-GridOS-Role"
	permissionsHeader = "X-GridOS-Permissions"
	siteLocation      = "site_location"
)

type Service struct {
	store             EventStore
	twin              *fleet.Twin
	sites             []*gridosv1.AuthorizedSite
	now               func() time.Time
	reports           reporting.Source
	startWorkflow     func(context.Context, string, dispatchWorkflowInput) error
	approveWorkflow   func(context.Context, string, dispatchWorkflowApproval) error
	launchWorkflow    func(context.Context, string, *gridosv1.LaunchEventRequest) error
	emergencyWorkflow func(context.Context, string, dispatchWorkflowEmergencyStop) error
}

func (service *Service) SetReportSource(source reporting.Source) {
	service.reports = source
}

func (service *Service) SetWorkflowClient(workflows client.Client, taskQueue string) {
	service.startWorkflow = func(ctx context.Context, eventID string, input dispatchWorkflowInput) error {
		_, err := workflows.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID: eventID, TaskQueue: taskQueue, WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		}, "Workflow", input)
		return err
	}
	service.approveWorkflow = func(ctx context.Context, eventID string, approval dispatchWorkflowApproval) error {
		return workflows.SignalWorkflow(ctx, eventID, "", approveEventSignal, approval)
	}
	service.launchWorkflow = func(ctx context.Context, eventID string, launch *gridosv1.LaunchEventRequest) error {
		return workflows.SignalWorkflow(ctx, eventID, "", launchEventSignal, launch)
	}
	service.emergencyWorkflow = func(ctx context.Context, eventID string, stop dispatchWorkflowEmergencyStop) error {
		return workflows.SignalWorkflow(ctx, eventID, "", emergencyStopSignal, stop)
	}
}

func (service *Service) RuntimeReady() bool {
	return service.startWorkflow != nil && service.approveWorkflow != nil && service.launchWorkflow != nil && service.emergencyWorkflow != nil && service.reports != nil
}

func (service *Service) RequestEmergencyStop(ctx context.Context, eventID, requestedBy string) error {
	if eventID == "" || requestedBy == "" {
		return errors.New("event ID and requester required")
	}
	if service.emergencyWorkflow == nil {
		return errors.New("workflow client required")
	}
	return service.emergencyWorkflow(ctx, eventID, dispatchWorkflowEmergencyStop{RequestedBy: requestedBy})
}

func NewService(store EventStore, twin *fleet.Twin, sites []*gridosv1.AuthorizedSite, now func() time.Time) *Service {
	return &Service{store: store, twin: twin, sites: sites, now: now}
}

func NewHandler(service *Service) http.Handler {
	return NewControlHandler(service, nil, "")
}

func NewControlHandler(service *Service, telemetry gridosv1connect.TelemetryServiceHandler, telemetryToken string, eventServices ...gridosv1connect.EventsServiceHandler) http.Handler {
	mux := http.NewServeMux()
	fleetPath, fleetHandler := gridosv1connect.NewFleetServiceHandler(service)
	dispatchPath, dispatchHandler := gridosv1connect.NewDispatchServiceHandler(service)
	mux.Handle(fleetPath, fleetHandler)
	mux.Handle(dispatchPath, dispatchHandler)
	if telemetry != nil {
		telemetryPath, telemetryHandler := gridosv1connect.NewTelemetryServiceHandler(telemetry)
		mux.Handle(telemetryPath, authorizeToken(telemetryHandler, telemetryToken))
	}
	for _, events := range eventServices {
		eventsPath, eventsHandler := gridosv1connect.NewEventsServiceHandler(events)
		mux.Handle(eventsPath, eventsHandler)
	}
	return mux
}

func authorizeToken(next http.Handler, expected string) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		actual := request.Header.Get("Authorization")
		if expected == "" || len(actual) != len(expected) || subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
			http.Error(response, "authorization required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(response, request)
	})
}

func (service *Service) GetFleetSummary(_ context.Context, request *connect.Request[gridosv1.GetFleetSummaryRequest]) (*connect.Response[gridosv1.GetFleetSummaryResponse], error) {
	if err := authorize(request.Header(), "operator", "approver", "analyst", "partner", "service"); err != nil {
		return nil, err
	}
	now := service.now()
	aggregate := service.twin.Aggregate(now)
	states := service.twin.Sites(now)
	installedMW, installedMWh := installedCapacity(service.sites)
	provenance := siteProvenanceMix(service.sites)
	dispatchableProvenance := aggregate.DispatchableMW.ProvenanceMix
	if len(dispatchableProvenance) == 0 {
		dispatchableProvenance = provenance
	}
	summary := &gridosv1.FleetSummary{
		InstalledMw:                 quantity(installedMW, now, 0, provenance),
		InstalledMwh:                quantity(installedMWh, now, 0, provenance),
		DispatchableNowMw:           quantity(aggregate.DispatchableMW.Value, now, aggregate.DispatchableMW.Freshness, dispatchableProvenance),
		ForecastDispatchableMw:      quantity(aggregate.DispatchableMW.Value, now, aggregate.DispatchableMW.Freshness, dispatchableProvenance),
		ReservedForBackupMwh:        quantity(max(installedMWh-aggregate.DispatchableMWh.Value, 0), now, aggregate.DispatchableMWh.Freshness, provenance),
		OperatingStateCounts:        operatingCounts(states, now, provenance),
		AvailabilityStateCounts:     availabilityCounts(states, now, provenance),
		CommunicationsHealthCounts:  healthCounts(states, now, provenance),
		AcknowledgementHealthCounts: healthCounts(states, now, provenance),
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
	now := service.now()
	locations, err := aggregateSites(selected, service.twin.Sites(now), now)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&gridosv1.ListSitesResponse{Sites: locations}), nil
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
	if service.startWorkflow == nil {
		return connect.NewResponse(&gridosv1.CreateEventRequestResponse{Event: event}), nil
	}
	err = service.startWorkflow(ctx, event.GetEventId(), dispatchWorkflowInput{EventID: event.GetEventId(), Request: eventRequest})
	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if err != nil && !errors.As(err, &alreadyStarted) {
		return nil, connect.NewError(connect.CodeUnavailable, err)
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
	response := &gridosv1.GetEventResponse{Event: event, Exclusions: groups}
	if store, ok := service.store.(LifecycleStore); ok {
		violations, violationErr := store.Violations(ctx, request.Msg.GetEventId())
		if violationErr != nil {
			return nil, connect.NewError(connect.CodeInternal, violationErr)
		}
		for _, violation := range violations {
			response.SafetyViolations = append(response.SafetyViolations, &gridosv1.SafetyViolation{Code: violation.Code})
		}
	}
	if service.reports != nil && event.GetPlanVersion() > 0 {
		report, reportErr := reporting.Build(ctx, service.reports, request.Msg.GetEventId())
		if reportErr != nil {
			return nil, connect.NewError(connect.CodeInternal, reportErr)
		}
		response.Report = eventReport(report)
	}
	return connect.NewResponse(response), nil
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
	if service.approveWorkflow != nil {
		err = service.approveWorkflow(ctx, request.Msg.GetEventId(), dispatchWorkflowApproval{ApprovedBy: request.Msg.GetApprovedBy()})
		if err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
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
	current, _, err := service.store.Get(ctx, request.Msg.GetEventId())
	if err != nil {
		return nil, storeError(err)
	}
	if current.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_APPROVED || current.GetPlanVersion() != request.Msg.GetPlanVersion() {
		return nil, storeError(ErrInvalidState)
	}
	if service.launchWorkflow == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("workflow client required"))
	}
	if err = service.launchWorkflow(ctx, request.Msg.GetEventId(), request.Msg); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&gridosv1.LaunchEventResponse{Event: current}), nil
}

const (
	approveEventSignal  = "approve-event"
	launchEventSignal   = "launch-event"
	emergencyStopSignal = "emergency-stop"
)

type dispatchWorkflowInput struct {
	EventID string
	Request *gridosv1.EventRequest
}

type dispatchWorkflowApproval struct {
	ApprovedBy string
}

type dispatchWorkflowEmergencyStop struct {
	RequestedBy string
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

func eventReport(source reporting.EventReport) *gridosv1.BasicEventReport {
	exclusions := make([]*gridosv1.ExclusionReasonGroup, 0, len(source.ExcludedByReason))
	for name, count := range source.ExcludedByReason {
		reason := gridosv1.ExclusionReason(gridosv1.ExclusionReason_value[name])
		exclusions = append(exclusions, &gridosv1.ExclusionReasonGroup{Reason: reason, Count: count})
	}
	sort.Slice(exclusions, func(i, j int) bool { return exclusions[i].GetReason() < exclusions[j].GetReason() })
	report := &gridosv1.BasicEventReport{
		RequestedMw: source.RequestedMW, ApprovedMw: source.ApprovedMW, CommandedMw: source.CommandedMW, AcknowledgedMw: source.AcknowledgedMW,
		Exclusions: exclusions, Provenance: source.Provenance, PolicyVersion: source.Versions.Policy, SolverVersion: source.Versions.Solver, ModelVersion: source.Versions.Model,
	}
	if source.Delivered == nil {
		return report
	}
	report.DeliveredMw = source.Delivered.DeliveredMW
	report.DeliveredMwh = source.Delivered.DeliveredMWh
	report.TrackingErrorMw = source.Delivered.TrackingErrorMW
	report.ResponseLatency = durationpb.New(source.Delivered.ResponseLatency)
	report.Completeness = source.Delivered.Completeness
	report.RespondedDevices = uint64(source.Delivered.Responded)
	report.CommandedDevices = uint64(source.Delivered.Commanded)
	for _, interval := range source.Delivered.UncertainIntervals {
		uncertain := &gridosv1.UncertainDeliveryInterval{DeviceId: interval.DeviceID, BeginTime: timestamppb.New(interval.Begin), EndTime: timestamppb.New(interval.End)}
		if interval.Bounds != nil {
			uncertain.LowerKw = interval.Bounds.LowerKW
			uncertain.UpperKw = interval.Bounds.UpperKW
		}
		report.UncertainIntervals = append(report.UncertainIntervals, uncertain)
	}
	return report
}

package member

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/policy"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	pool   *pgxpool.Pool
	twin   *fleet.Twin
	sites  map[string]*gridosv1.AuthorizedSite
	policy *policy.Store
	now    func() time.Time
}

func NewService(pool *pgxpool.Pool, twin *fleet.Twin, sites []*gridosv1.AuthorizedSite, now func() time.Time) *Service {
	indexed := make(map[string]*gridosv1.AuthorizedSite, len(sites))
	for _, site := range sites {
		indexed[site.GetSite().GetSiteId()] = site
	}
	return &Service{pool: pool, twin: twin, sites: indexed, policy: policy.New(pool), now: now}
}

func (service *Service) authorize(ctx context.Context, role, principal, memberID, siteID string, readOnly bool) error {
	if role == "" {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("role required"))
	}
	if memberID == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("member required"))
	}
	if role != "member" && (!readOnly || role != "operator") {
		return connect.NewError(connect.CodePermissionDenied, errors.New("member role required"))
	}
	if role == "member" && (principal == "" || principal != memberID) {
		return connect.NewError(connect.CodePermissionDenied, errors.New("member identity mismatch"))
	}
	var bound bool
	var err error
	if siteID == "" {
		err = service.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM member_sites WHERE member_id = $1)`, memberID).Scan(&bound)
	} else {
		err = service.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM member_sites WHERE member_id = $1 AND site_id = $2)`, memberID, siteID).Scan(&bound)
	}
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !bound {
		return connect.NewError(connect.CodePermissionDenied, errors.New("site is not bound to member"))
	}
	if siteID != "" && service.sites[siteID] == nil {
		return connect.NewError(connect.CodeNotFound, errors.New("site unavailable"))
	}
	return nil
}

func (service *Service) GetMemberStatus(ctx context.Context, request *connect.Request[gridosv1.GetMemberStatusRequest]) (*connect.Response[gridosv1.GetMemberStatusResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	memberID, siteID := request.Msg.GetMemberId(), request.Msg.GetSiteId()
	if siteID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("site required"))
	}
	if err := service.authorize(ctx, request.Header().Get("X-GridOS-Role"), request.Header().Get("X-GridOS-Member-ID"), memberID, siteID, true); err != nil {
		return nil, err
	}
	state, found := service.twin.Site(siteID, service.now())
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("site telemetry unavailable"))
	}
	result := &gridosv1.GetMemberStatusResponse{MemberId: memberID, SiteId: siteID,
		OperatingState: operatingState(state.OperatingState), Availability: availability(state.Availability),
		BackupHoursCurrent: state.BackupHoursCurrent, BackupHours_750W: state.BackupHours750W,
		ObservedAt: timestamppb.New(state.ObservedAt)}
	var capacity float64
	for _, device := range service.sites[siteID].GetDevices() {
		capacity += device.GetBatteryParameters().GetUsableEnergyKwh()
	}
	if capacity > 0 {
		result.StateOfEnergyPercent = 100 * state.EnergyKWh / capacity
	}
	plan, err := service.policy.Current(ctx, memberID, service.now())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if plan != nil {
		result.CurrentPlan = selectedPlan(plan)
		reserve, err := service.policy.ReserveAt(ctx, memberID, service.now())
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		result.EffectiveReservePercent = reserve.EffectivePercent
	}
	return connect.NewResponse(result), nil
}

func selectedPlan(plan *policy.Plan) *gridosv1.SelectedMemberPlan {
	return &gridosv1.SelectedMemberPlan{SelectionId: plan.ID, OfferId: plan.OfferID,
		CatalogVersion: plan.CatalogVersion, MemberPlanId: plan.MemberPlanID,
		DisplayName: plan.DisplayName, ReserveFloorPercent: plan.ReserveFloorPercent,
		PolicyVersion: plan.PolicyVersion, EffectiveAt: timestamppb.New(plan.EffectiveAt),
		EnergyMonthlyChargeCents:  plan.EnergyMonthlyChargeCents,
		BatteryMonthlyChargeCents: plan.BatteryMonthlyChargeCents,
		FlexibilityRewardCents:    plan.FlexibilityRewardCents, TermsKnown: plan.OfferID != ""}
}

func operatingState(state fleet.OperatingState) gridosv1.FleetOperatingState {
	switch state {
	case fleet.OnGrid:
		return gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_ON_GRID
	case fleet.OffGridOutage:
		return gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_OUTAGE
	case fleet.OffGridNoHomePower:
		return gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_NO_HOME_POWER
	case fleet.OffGridOvercurrent:
		return gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_OVERCURRENT
	case fleet.OffGridOvercurrentStandby:
		return gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_OVERCURRENT_STANDBY
	case fleet.TelemetryUnavailable:
		return gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_TELEMETRY_UNAVAILABLE
	default:
		return gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_UNSPECIFIED
	}
}

func availability(state fleet.Availability) gridosv1.FleetAvailabilityState {
	switch state {
	case fleet.Online:
		return gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_ONLINE
	case fleet.Offline:
		return gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_OFFLINE
	case fleet.Degraded:
		return gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_DEGRADED
	case fleet.Stale:
		return gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_STALE
	case fleet.Maintenance:
		return gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_MAINTENANCE
	default:
		return gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_UNSPECIFIED
	}
}

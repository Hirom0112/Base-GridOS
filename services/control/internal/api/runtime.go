package api

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/safety"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ConnectOptimizer struct {
	client gridosv1connect.OptimizationServiceClient
}

func NewConnectOptimizer(client gridosv1connect.OptimizationServiceClient) *ConnectOptimizer {
	return &ConnectOptimizer{client: client}
}

func (optimizer *ConnectOptimizer) Optimize(ctx context.Context, request *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error) {
	response, err := optimizer.client.Optimize(ctx, connect.NewRequest(&gridosv1.OptimizeRequest{Request: request}))
	if err != nil {
		return nil, err
	}
	return response.Msg.GetPlan(), nil
}

type StoredApprovalGate struct {
	events EventStore
}

func NewStoredApprovalGate(events EventStore) *StoredApprovalGate {
	return &StoredApprovalGate{events: events}
}

func (gate *StoredApprovalGate) Require(ctx context.Context, eventID string, planVersion uint64) error {
	event, _, err := gate.events.Get(ctx, eventID)
	if err != nil {
		return err
	}
	if event.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_APPROVED || event.GetPlanVersion() != planVersion {
		return ErrApprovalRequired
	}
	return nil
}

type CommandPipeline struct {
	pool      *pgxpool.Pool
	publisher commandPublisher
}

type commandPublisher interface {
	storage.OutboxPublisher
	PublishBatch(context.Context) error
}

func NewCommandPipeline(pool *pgxpool.Pool, publisher commandPublisher) *CommandPipeline {
	return &CommandPipeline{pool: pool, publisher: publisher}
}

func (pipeline *CommandPipeline) Persist(ctx context.Context, commands []storage.CommandIntent) error {
	for _, command := range commands {
		if err := storage.InsertCommand(ctx, pipeline.pool, command); err != nil {
			return err
		}
	}
	return nil
}

func (pipeline *CommandPipeline) Publish(ctx context.Context, command storage.ClaimedCommand) error {
	return pipeline.publisher.Publish(ctx, command)
}

func (pipeline *CommandPipeline) PublishAll(ctx context.Context) error {
	return pipeline.publisher.PublishBatch(ctx)
}

type FleetSnapshotter struct {
	twin      *fleet.Twin
	sites     []*gridosv1.AuthorizedSite
	now       func() time.Time
	pool      *pgxpool.Pool
	telemetry *fleet.TelemetryTwin
}

func NewFleetSnapshotter(twin *fleet.Twin, sites []*gridosv1.AuthorizedSite, now func() time.Time) *FleetSnapshotter {
	return &FleetSnapshotter{twin: twin, sites: sites, now: now}
}

func NewDurableFleetSnapshotter(pool *pgxpool.Pool, twin *fleet.Twin, telemetry *fleet.TelemetryTwin, sites []*gridosv1.AuthorizedSite, now func() time.Time) *FleetSnapshotter {
	return &FleetSnapshotter{pool: pool, twin: twin, telemetry: telemetry, sites: sites, now: now}
}

func (snapshotter *FleetSnapshotter) Freeze(ctx context.Context, event *gridosv1.DispatchEvent, request *gridosv1.EventRequest) (FrozenSnapshot, error) {
	if event == nil || request == nil || request.GetBeginTime() == nil || request.GetEndTime() == nil {
		return FrozenSnapshot{}, errors.New("event and dispatch window required")
	}
	if err := snapshotter.loadTelemetry(ctx); err != nil {
		return FrozenSnapshot{}, err
	}
	now := snapshotter.now()
	planVersion := event.GetPlanVersion() + 1
	states := make(map[string]fleet.SiteState)
	for _, state := range snapshotter.twin.Sites(now) {
		states[state.SiteID] = state
	}
	optimization := &gridosv1.OptimizationRequest{
		RequestId: request.GetRequestId(), EventId: event.GetEventId(), PlanVersion: planVersion, RequestedAt: timestamppb.New(now),
		CorrelationId: request.GetCorrelationId(), Budget: durationpb.New(5 * time.Second), MeasurementBoundary: request.GetMeasurementBoundary(),
		Intervals:           []*gridosv1.OptimizationInterval{{BeginTime: request.GetBeginTime(), EndTime: request.GetEndTime(), TargetKw: request.GetTargetKw()}},
		EligibilitySnapshot: &gridosv1.EligibilitySnapshot{EventId: event.GetEventId(), CapturedAt: timestamppb.New(now), PolicyVersion: "fleet-file"},
		ReservePolicy:       &gridosv1.ReservePolicy{PolicyVersion: "fleet-file", EffectiveAt: timestamppb.New(now), ExpiresAt: request.GetEndTime()},
	}
	canonical := safety.CanonicalState{Now: now, Boundary: safety.MeterNetExport, PolicyVersion: "fleet-file", ExpectedGeneration: int64(planVersion), Devices: make(map[string]safety.DeviceState)}
	for _, site := range snapshotter.sites {
		state := states[site.GetSite().GetSiteId()]
		for _, device := range site.GetDevices() {
			parameters := device.GetBatteryParameters()
			energy := state.EnergyKWh
			available := state.OperatingState == fleet.OnGrid && state.Availability == fleet.Online
			optimization.Devices = append(optimization.Devices, &gridosv1.DeviceState{
				DeviceId: device.GetDeviceId(), UsableEnergyKwh: parameters.GetUsableEnergyKwh(), EnergyKwh: energy,
				HardwareFloorKwh: state.ReserveKWh, EffectiveReserveKwh: state.ReserveKWh,
				MaxChargeKw: parameters.GetMaxChargeKw(), MaxDischargeKw: parameters.GetMaxDischargeKw(),
				ChargeEfficiency: parameters.GetChargeEfficiency(), DischargeEfficiency: parameters.GetDischargeEfficiency(),
				AvailabilityProbability: boolFloat(available), Stale: state.Availability == fleet.Stale, TelemetryObservedAt: timestamppb.New(state.ObservedAt), LoadZone: site.GetSite().GetLoadZone(),
			})
			if available {
				optimization.EligibilitySnapshot.EligibleDeviceIds = append(optimization.EligibilitySnapshot.EligibleDeviceIds, device.GetDeviceId())
			}
			observedAt := state.ObservedAt
			canonical.Devices[device.GetDeviceId()] = safety.DeviceState{
				EnergyKWh: &energy, UsableCapacityKWh: parameters.GetUsableEnergyKwh(), HardwareReserveKWh: state.ReserveKWh, PlanReserveKWh: state.ReserveKWh,
				MaxChargeKW: parameters.GetMaxChargeKw(), MaxDischargeKW: parameters.GetMaxDischargeKw(), ChargeEfficiency: parameters.GetChargeEfficiency(), DischargeEfficiency: parameters.GetDischargeEfficiency(),
				Available: available, TelemetryAt: &observedAt, FreshnessLimit: 30 * time.Second, MeterExportLimitKW: parameters.GetMaxDischargeKw(), InterconnectionLimitKW: parameters.GetMaxDischargeKw(),
			}
		}
	}
	return FrozenSnapshot{Optimization: optimization, Canonical: canonical}, nil
}

func (snapshotter *FleetSnapshotter) loadTelemetry(ctx context.Context) error {
	if snapshotter.pool == nil || snapshotter.telemetry == nil {
		return nil
	}
	rows, err := snapshotter.pool.Query(ctx, `SELECT DISTINCT ON (actor_id) new_values
		FROM audit_journal WHERE action = 'TELEMETRY_RECEIVED'
		ORDER BY actor_id, occurred_at DESC, sequence DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var values []byte
		if err = rows.Scan(&values); err != nil {
			return err
		}
		var observation gridosv1.TelemetryObservation
		if err = protojson.Unmarshal(values, &observation); err != nil {
			return err
		}
		snapshotter.telemetry.Accept(&observation)
	}
	return rows.Err()
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

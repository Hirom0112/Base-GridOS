package events

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/reconciliation"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type StopRequester interface {
	RequestEmergencyStop(context.Context, string, string) error
}

type PostgresSource struct {
	pool        *pgxpool.Pool
	events      *storage.PostgresEventStore
	stops       StopRequester
	deviceCells map[string]string
	cellCounts  map[string]uint64
	now         func() time.Time
	maxGap      time.Duration
}

func NewPostgresSource(pool *pgxpool.Pool, stops StopRequester, sites []*gridosv1.AuthorizedSite, now func() time.Time, maxGap time.Duration) *PostgresSource {
	deviceCells := make(map[string]string)
	cellCounts := make(map[string]uint64)
	for _, site := range sites {
		cellID := site.GetSite().GetH3Cell()
		cellCounts[cellID]++
		for _, device := range site.GetDevices() {
			deviceCells[device.GetDeviceId()] = cellID
		}
	}
	return &PostgresSource{pool: pool, events: storage.NewPostgresEventStore(pool), stops: stops, deviceCells: deviceCells, cellCounts: cellCounts, now: now, maxGap: maxGap}
}

func (source *PostgresSource) Snapshot(ctx context.Context, eventID string) (*gridosv1.WatchEventResponse, error) {
	event, _, err := source.events.Get(ctx, eventID)
	if err != nil {
		return nil, err
	}
	now := source.now()
	commands, err := source.commandPower(ctx, eventID)
	if err != nil {
		return nil, err
	}
	delivered, err := reconciliation.LoadDelivered(ctx, source.pool, eventID)
	if err != nil {
		return nil, err
	}
	cells, err := reconciliation.DeliveredByCell(ctx, source.pool, eventID, source.deviceCells, now, source.maxGap)
	if err != nil {
		return nil, err
	}
	exceptions, err := source.TimelineExceptions(ctx, eventID)
	if err != nil {
		return nil, err
	}
	fleet := &gridosv1.EventPowerAggregate{SentMw: commands.sent / 1000, AcknowledgedMw: commands.acknowledged / 1000, Metadata: source.metadata(now, uint64(len(source.deviceCells)))}
	if delivered != nil {
		fleet.DeliveredMw = delivered.DeliveredMW
		fleet.DeliveredState = gridosv1.ValueState_VALUE_STATE_PRESENT
		fleet.UncertaintyIntervals = uncertaintyMessages(delivered.UncertainIntervals)
	} else {
		fleet.DeliveredState = gridosv1.ValueState_VALUE_STATE_MISSING
	}
	byCell := make(map[string]*gridosv1.H3EventPowerAggregate)
	for cellID, power := range commands.cells {
		byCell[cellID] = &gridosv1.H3EventPowerAggregate{H3Cell: cellID, Power: &gridosv1.EventPowerAggregate{
			SentMw: power.sent / 1000, AcknowledgedMw: power.acknowledged / 1000, Metadata: source.metadata(now, source.cellCounts[cellID]),
		}, Metadata: source.metadata(now, source.cellCounts[cellID])}
	}
	for _, cell := range cells {
		aggregate := byCell[cell.H3Cell]
		if aggregate == nil {
			aggregate = &gridosv1.H3EventPowerAggregate{H3Cell: cell.H3Cell, Power: &gridosv1.EventPowerAggregate{Metadata: source.metadata(now, source.cellCounts[cell.H3Cell])}, Metadata: source.metadata(now, source.cellCounts[cell.H3Cell])}
			byCell[cell.H3Cell] = aggregate
		}
		if cell.Known {
			aggregate.Power.DeliveredMw = cell.DeliveredMW
			aggregate.Power.DeliveredState = gridosv1.ValueState_VALUE_STATE_PRESENT
		} else {
			aggregate.Power.DeliveredState = gridosv1.ValueState_VALUE_STATE_MISSING
		}
		aggregate.Power.UncertaintyIntervals = uncertaintyMessages(cell.UncertainIntervals)
	}
	return &gridosv1.WatchEventResponse{Event: event, Fleet: fleet, H3: sortedCells(byCell), ObservedAt: timestamppb.New(now), Exceptions: exceptions}, nil
}

type power struct {
	sent         float64
	acknowledged float64
}

type commandPower struct {
	power
	cells map[string]power
}

func (source *PostgresSource) commandPower(ctx context.Context, eventID string) (commandPower, error) {
	result := commandPower{cells: make(map[string]power)}
	rows, err := source.pool.Query(ctx, `SELECT intent.device_id, intent.setpoint_kw, latest.state,
		EXISTS (SELECT 1 FROM command_acknowledgements WHERE command_id = intent.command_id AND receipt_status = 'ACCEPTED')
		FROM command_intents AS intent
		JOIN LATERAL (SELECT state FROM command_states WHERE command_id = intent.command_id ORDER BY recorded_at DESC LIMIT 1) AS latest ON true
		WHERE intent.event_id = $1`, eventID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var deviceID, state string
		var setpoint float64
		var acknowledged bool
		if err = rows.Scan(&deviceID, &setpoint, &state, &acknowledged); err != nil {
			return result, err
		}
		cellID := source.deviceCells[deviceID]
		cell := result.cells[cellID]
		if state != "PERSISTED" {
			result.sent += setpoint
			cell.sent += setpoint
		}
		if acknowledged {
			result.acknowledged += setpoint
			cell.acknowledged += setpoint
		}
		result.cells[cellID] = cell
	}
	return result, rows.Err()
}

func (source *PostgresSource) Timeline(ctx context.Context, eventID string) ([]*gridosv1.EventTimelineEntry, error) {
	rows, err := source.pool.Query(ctx, `SELECT sequence, occurred_at, actor_id, action, previous_values, new_values
		FROM audit_journal WHERE resource_id = $1 OR resource_id IN (SELECT command_id FROM command_intents WHERE event_id = $1)
		ORDER BY sequence`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]*gridosv1.EventTimelineEntry, 0)
	for rows.Next() {
		var sequence uint64
		var occurredAt time.Time
		var actorID, action string
		var previousValues, newValues []byte
		if err = rows.Scan(&sequence, &occurredAt, &actorID, &action, &previousValues, &newValues); err != nil {
			return nil, err
		}
		previous := auditValues(previousValues)
		next := auditValues(newValues)
		entries = append(entries, &gridosv1.EventTimelineEntry{
			Sequence: sequence, OccurredAt: timestamppb.New(occurredAt), ActorId: actorID, Action: action,
			PreviousState: eventState(previous.State), State: eventState(next.State), Reason: next.Reason,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

type transitionValues struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

func auditValues(values []byte) transitionValues {
	var decoded transitionValues
	_ = json.Unmarshal(values, &decoded)
	return decoded
}

func eventState(value string) gridosv1.DispatchEventState {
	return gridosv1.DispatchEventState(gridosv1.DispatchEventState_value["DISPATCH_EVENT_STATE_"+strings.ToUpper(value)])
}

func (source *PostgresSource) RequestStop(ctx context.Context, request *gridosv1.EmergencyStopRequest) (*gridosv1.EmergencyStopResponse, error) {
	tx, err := source.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var existing gridosv1.EmergencyStop
	var requestedAt time.Time
	err = tx.QueryRow(ctx, `SELECT emergency_stop_id, idempotency_key, event_id, requested_by, reason, requested_at, correlation_id
		FROM emergency_stops WHERE idempotency_key = $1`, request.GetIdempotencyKey()).Scan(
		&existing.EmergencyStopId, &existing.IdempotencyKey, &existing.EventId, &existing.RequestedBy, &existing.Reason,
		&requestedAt, &existing.CorrelationId,
	)
	if err == nil {
		existing.RequestedAt = timestamppb.New(requestedAt)
		return connectStop(&existing), tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err = source.requireStoppable(ctx, tx, request.GetEventId()); err != nil {
		return nil, err
	}
	stop := &gridosv1.EmergencyStop{
		EmergencyStopId: request.GetEventId() + ":" + request.GetIdempotencyKey(), IdempotencyKey: request.GetIdempotencyKey(),
		EventId: request.GetEventId(), RequestedBy: request.GetRequestedBy(), Reason: request.GetReason(), RequestedAt: request.GetRequestedAt(), CorrelationId: request.GetCorrelationId(),
	}
	_, err = tx.Exec(ctx, `INSERT INTO emergency_stops
		(emergency_stop_id, idempotency_key, event_id, requested_by, reason, requested_at, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, stop.GetEmergencyStopId(), stop.GetIdempotencyKey(), stop.GetEventId(), stop.GetRequestedBy(), stop.GetReason(), stop.GetRequestedAt().AsTime(), stop.GetCorrelationId())
	if err != nil {
		return nil, err
	}
	if err = source.stops.RequestEmergencyStop(ctx, stop.GetEventId(), stop.GetRequestedBy()); err != nil {
		return nil, err
	}
	values, err := protojson.Marshal(stop)
	if err != nil {
		return nil, err
	}
	if err = storage.AppendAudit(ctx, tx, storage.AuditRecord{OccurredAt: stop.GetRequestedAt().AsTime(), ActorID: stop.GetRequestedBy(), Action: "EMERGENCY_STOP_REQUESTED", ResourceID: stop.GetEventId(), NewValues: values, CorrelationID: stop.GetCorrelationId()}); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return connectStop(stop), nil
}

func (source *PostgresSource) requireStoppable(ctx context.Context, tx pgx.Tx, eventID string) error {
	var state string
	if err := tx.QueryRow(ctx, "SELECT state FROM dispatch_events WHERE event_id = $1 FOR SHARE", eventID).Scan(&state); err != nil {
		return err
	}
	allowed := map[string]bool{"SENT": true, "ACKNOWLEDGED_OR_UNCERTAIN": true, "EXECUTING": true, "VERIFIED": true, "RECONCILED": true, "REPORTED": true}
	if !allowed[state] {
		return errors.New("event has not sent commands")
	}
	return nil
}

func connectStop(stop *gridosv1.EmergencyStop) *gridosv1.EmergencyStopResponse {
	return &gridosv1.EmergencyStopResponse{EmergencyStop: stop, StopRequested: true}
}

func uncertaintyMessages(intervals []report.UncertainInterval) []*gridosv1.UncertaintyInterval {
	result := make([]*gridosv1.UncertaintyInterval, 0, len(intervals))
	for _, interval := range intervals {
		message := &gridosv1.UncertaintyInterval{DeviceId: interval.DeviceID, IntervalBeginTime: timestamppb.New(interval.Begin), IntervalEndTime: timestamppb.New(interval.End)}
		if interval.Bounds != nil {
			message.SignedFeasiblePowerLowerKw = interval.Bounds.LowerKW
			message.SignedFeasiblePowerUpperKw = interval.Bounds.UpperKW
		}
		result = append(result, message)
	}
	return result
}

func sortedCells(cells map[string]*gridosv1.H3EventPowerAggregate) []*gridosv1.H3EventPowerAggregate {
	result := make([]*gridosv1.H3EventPowerAggregate, 0, len(cells))
	for _, cell := range cells {
		result = append(result, cell)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].GetH3Cell() < result[right].GetH3Cell() })
	return result
}

func (source *PostgresSource) metadata(now time.Time, count uint64) *gridosv1.AggregateMetadata {
	return &gridosv1.AggregateMetadata{
		Timestamp: nowTimestamp(now), Freshness: durationpb.New(0),
		ProvenanceMix: []*gridosv1.ProvenanceShare{{Provenance: gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED, RecordCount: count}},
	}
}

func nowTimestamp(now time.Time) *timestamppb.Timestamp {
	return timestamppb.New(now)
}

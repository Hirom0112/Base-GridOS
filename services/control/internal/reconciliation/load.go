package reconciliation

import (
	"context"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
)

type storedEvent struct {
	eventID       string
	correlationID string
	boundary      string
	begin         time.Time
	end           time.Time
	targetKW      float64
}

var boundaryPower = map[string]func(*gridosv1.PowerFlow) float64{
	"METER_NET_EXPORT": func(flow *gridosv1.PowerFlow) float64 { return -flow.GetFromGridKw() },
	"BATTERY_TERMINAL": func(flow *gridosv1.PowerFlow) float64 { return flow.GetFromStorageKw() },
}

func loadEvent(ctx context.Context, pool *pgxpool.Pool, eventID string) (storedEvent, error) {
	stored := storedEvent{eventID: eventID}
	err := pool.QueryRow(ctx, `SELECT event.correlation_id, request.measurement_boundary, request.begin_time, request.end_time, request.target_kw
		FROM dispatch_events AS event JOIN dispatch_requests AS request USING (request_id)
		WHERE event.event_id = $1`, eventID).Scan(&stored.correlationID, &stored.boundary, &stored.begin, &stored.end, &stored.targetKW)
	stored.begin, stored.end = stored.begin.UTC(), stored.end.UTC()
	return stored, err
}

func loadCommands(ctx context.Context, pool *pgxpool.Pool, event *Event, eventID string) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT intent.command_id, intent.device_id, intent.setpoint_kw, intent.effective_at, intent.expires_at,
			latest.state, latest.recorded_at,
			EXISTS (SELECT 1 FROM command_acknowledgements AS acknowledgement
				WHERE acknowledgement.command_id = intent.command_id AND acknowledgement.receipt_status = 'ACCEPTED')
		FROM command_intents AS intent
		JOIN LATERAL (SELECT state, recorded_at FROM command_states
			WHERE command_id = intent.command_id ORDER BY recorded_at DESC LIMIT 1) AS latest ON true
		WHERE intent.event_id = $1
		ORDER BY intent.generation, intent.issued_at, intent.command_id`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	commandIDs := make([]string, 0)
	for rows.Next() {
		var command Command
		var deviceID, state string
		var recordedAt time.Time
		var acknowledged bool
		if err = rows.Scan(&command.ID, &deviceID, &command.SetpointKW, &command.EffectiveAt, &command.ExpiresAt, &state, &recordedAt, &acknowledged); err != nil {
			return nil, err
		}
		command.EffectiveAt, command.ExpiresAt = command.EffectiveAt.UTC(), command.ExpiresAt.UTC()
		event.Command(deviceID, command)
		if acknowledged {
			event.Acknowledge(command.ID)
		}
		if state == "EXPIRED" || state == "CANCELLED" || state == "REJECTED" {
			event.Expire(command.ID, recordedAt.UTC())
		}
		commandIDs = append(commandIDs, command.ID)
	}
	return commandIDs, rows.Err()
}

func loadTelemetry(ctx context.Context, pool *pgxpool.Pool, event *Event, boundary string) error {
	window := event.measurement
	rows, err := pool.Query(ctx, `SELECT payload FROM telemetry_observations
		WHERE device_id = ANY($1) AND observed_at BETWEEN $2 AND $3
		ORDER BY observed_at, sequence`, event.deviceIDs(), window.Begin.Add(-window.MaxGap), window.End.Add(window.MaxGap))
	if err != nil {
		return err
	}
	defer rows.Close()
	power := boundaryPower[boundary]
	for rows.Next() {
		var values []byte
		if err = rows.Scan(&values); err != nil {
			return err
		}
		var observation gridosv1.TelemetryObservation
		if err = protojson.Unmarshal(values, &observation); err != nil {
			return err
		}
		if observation.GetValueState() != gridosv1.ValueState_VALUE_STATE_PRESENT || observation.GetPowerFlow() == nil {
			continue
		}
		event.Observe(observation.GetDeviceId(), Telemetry{PowerKW: power(observation.GetPowerFlow()), ObservedAt: observation.GetObservationTime().AsTime()})
	}
	return rows.Err()
}

func loadUncertain(ctx context.Context, pool *pgxpool.Pool, commandIDs []string) ([]report.UncertainInterval, error) {
	rows, err := pool.Query(ctx, `SELECT device_id, interval_begin_time, interval_end_time,
			signed_feasible_power_lower_kw, signed_feasible_power_upper_kw
		FROM uncertainty_intervals WHERE possibly_accepted_command_id = ANY($1)
		ORDER BY device_id, interval_begin_time`, commandIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	intervals := make([]report.UncertainInterval, 0)
	for rows.Next() {
		interval := report.UncertainInterval{Bounds: &report.PowerBounds{}}
		if err = rows.Scan(&interval.DeviceID, &interval.Begin, &interval.End, &interval.Bounds.LowerKW, &interval.Bounds.UpperKW); err != nil {
			return nil, err
		}
		interval.Begin, interval.End = interval.Begin.UTC(), interval.End.UTC()
		intervals = append(intervals, interval)
	}
	return intervals, rows.Err()
}

package reconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	verifiedAction = "DELIVERY_VERIFIED"
	baselineMethod = "MEASURED_AT_BOUNDARY"
)

func summarize(verification Verification, uncertain []report.UncertainInterval) report.Delivered {
	delivered := report.Delivered{
		DeliveredMWh:       verification.DeliveredKWh / 1000,
		ResponseLatency:    verification.Response.Latency,
		Responded:          verification.Response.Responded,
		Commanded:          verification.Response.Commanded,
		UncertainIntervals: make([]report.UncertainInterval, 0, len(verification.Gaps)+len(uncertain)),
	}
	measured := 0
	for _, interval := range verification.Intervals {
		if interval.Measured <= 0 {
			continue
		}
		measured++
		delivered.DeliveredMW += interval.DeliveredKW / 1000
		delivered.TrackingErrorMW += interval.TrackingErrorKW / 1000
	}
	if measured > 0 {
		delivered.DeliveredMW /= float64(measured)
		delivered.TrackingErrorMW /= float64(measured)
	}
	if verification.Expected > 0 {
		delivered.Completeness = float64(verification.Measured) / float64(verification.Expected)
	}
	for _, gap := range verification.Gaps {
		delivered.UncertainIntervals = append(delivered.UncertainIntervals, report.UncertainInterval{DeviceID: gap.DeviceID, Begin: gap.Begin, End: gap.End})
	}
	delivered.UncertainIntervals = append(delivered.UncertainIntervals, uncertain...)
	return delivered
}

func persist(ctx context.Context, pool *pgxpool.Pool, stored storedEvent, verification Verification, delivered report.Delivered, now time.Time) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, interval := range verification.Intervals {
		confidence := 0.0
		if interval.Expected > 0 {
			confidence = float64(interval.Measured) / float64(interval.Expected)
		}
		_, err = tx.Exec(ctx, `INSERT INTO verification_summaries (
				verification_id, event_id, interval_begin_time, interval_end_time, requested_kw, commanded_kw,
				delivered_kw, tracking_error_kw, confidence, baseline_method, measurement_boundary, correlation_id, measured_delivered_kwh
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (verification_id) DO UPDATE SET
				interval_end_time = EXCLUDED.interval_end_time, requested_kw = EXCLUDED.requested_kw, commanded_kw = EXCLUDED.commanded_kw, delivered_kw = EXCLUDED.delivered_kw,
				tracking_error_kw = EXCLUDED.tracking_error_kw, confidence = EXCLUDED.confidence,
				measured_delivered_kwh = EXCLUDED.measured_delivered_kwh`,
			stored.eventID+":"+interval.Begin.Format(time.RFC3339), stored.eventID, interval.Begin, interval.End, stored.targetKW,
			interval.CommandedKWh/interval.End.Sub(interval.Begin).Hours(), interval.DeliveredKW, interval.TrackingErrorKW,
			confidence, baselineMethod, stored.boundary, stored.correlationID, interval.DeliveredKWh)
		if err != nil {
			return err
		}
	}
	values, err := json.Marshal(delivered)
	if err != nil {
		return err
	}
	err = storage.AppendAudit(ctx, tx, storage.AuditRecord{
		OccurredAt: now, ActorID: "reconciliation", Action: verifiedAction, ResourceID: stored.eventID, NewValues: values, CorrelationID: stored.correlationID,
	})
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func LoadDelivered(ctx context.Context, pool *pgxpool.Pool, eventID string) (*report.Delivered, error) {
	var values []byte
	err := pool.QueryRow(ctx, `SELECT new_values FROM audit_journal
		WHERE resource_id = $1 AND action = $2 ORDER BY sequence DESC LIMIT 1`, eventID, verifiedAction).Scan(&values)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var delivered report.Delivered
	if err = json.Unmarshal(values, &delivered); err != nil {
		return nil, err
	}
	return &delivered, nil
}

package api

import (
	"context"
	"errors"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/reconciliation"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/jackc/pgx/v5/pgtype"
)

func plannedShortfalls(plan *gridosv1.DispatchPlan) ([]report.PlannedShortfall, error) {
	values := make([]report.PlannedShortfall, 0, len(plan.GetShortfalls()))
	for _, shortfall := range plan.GetShortfalls() {
		if shortfall == nil || shortfall.GetIntervalBeginTime() == nil || shortfall.GetIntervalEndTime() == nil || !shortfall.GetIntervalBeginTime().IsValid() || !shortfall.GetIntervalEndTime().IsValid() {
			return nil, errors.New("stored plan shortfall interval is invalid")
		}
		values = append(values, report.PlannedShortfall{
			Begin: shortfall.GetIntervalBeginTime().AsTime(), End: shortfall.GetIntervalEndTime().AsTime(),
			RequestedKW: shortfall.GetRequestedKw(), FeasibleKW: shortfall.GetFeasibleKw(), ShortfallKW: shortfall.GetShortfallKw(),
			Reasons: append([]string(nil), shortfall.GetReasons()...),
		})
	}
	return values, nil
}

func (source *PostgresReportSource) deliveryShortfalls(ctx context.Context, eventID string, begin, end time.Time, requestedKW float64) ([]report.DeliveryShortfall, error) {
	rows, err := source.pool.Query(ctx, `SELECT interval_begin_time, interval_end_time, requested_kw,
		measured_delivered_kwh, confidence FROM verification_summaries
		WHERE event_id = $1 ORDER BY interval_begin_time, verification_id`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]report.DeliveryShortfall, 0)
	for rows.Next() {
		var value report.DeliveryShortfall
		var measured pgtype.Float8
		var power float64
		if err := rows.Scan(&value.Begin, &value.End, &power, &measured, &value.Coverage); err != nil {
			return nil, err
		}
		value.RequestedKWh = power * value.End.Sub(value.Begin).Hours()
		value.ValueKind = "UNKNOWN"
		if value.Coverage > 0 && measured.Valid {
			value.MeasuredDeliveredKWh = &measured.Float64
			shortfall := value.RequestedKWh - measured.Float64
			value.ShortfallKWh = &shortfall
			value.ValueKind = "MEASURED"
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(values) > 0 {
		return values, nil
	}
	for at := begin; at.Before(end); at = at.Add(reconciliation.ReportingInterval) {
		intervalEnd := at.Add(reconciliation.ReportingInterval)
		if intervalEnd.After(end) {
			intervalEnd = end
		}
		values = append(values, report.DeliveryShortfall{Begin: at, End: intervalEnd,
			RequestedKWh: requestedKW * intervalEnd.Sub(at).Hours(), ValueKind: "UNKNOWN"})
	}
	return values, nil
}

package api

import (
	"context"
	"encoding/json"
	"errors"

	reporting "github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresReportSource struct {
	pool *pgxpool.Pool
}

func NewPostgresReportSource(pool *pgxpool.Pool) *PostgresReportSource {
	return &PostgresReportSource{pool: pool}
}

func (source *PostgresReportSource) EventReportData(ctx context.Context, eventID string) (reporting.StoredEvent, error) {
	var report reporting.StoredEvent
	var approved bool
	err := source.pool.QueryRow(ctx, `SELECT request.target_kw / 1000.0,
        EXISTS (SELECT 1 FROM operator_approvals WHERE event_id = event.event_id AND decision = 'APPROVED')
        FROM dispatch_events AS event JOIN dispatch_requests AS request USING (request_id)
        WHERE event.event_id = $1`, eventID).Scan(&report.RequestedMW, &approved)
	if err != nil {
		return report, err
	}
	if approved {
		report.ApprovedMW = report.RequestedMW
	}
	err = source.pool.QueryRow(ctx, `SELECT
        COALESCE((SELECT sum(setpoint_kw) / 1000.0 FROM command_intents WHERE event_id = $1), 0),
        COALESCE((SELECT sum(intent.setpoint_kw) / 1000.0
          FROM command_intents AS intent
          JOIN command_acknowledgements AS acknowledgement USING (command_id)
          WHERE intent.event_id = $1 AND acknowledgement.receipt_status = 'ACCEPTED'), 0)`, eventID).Scan(&report.CommandedMW, &report.AcknowledgedMW)
	if err != nil {
		return report, err
	}
	var exclusions []byte
	err = source.pool.QueryRow(ctx, `SELECT exclusions, policy_version
        FROM eligibility_snapshots WHERE event_id = $1 ORDER BY captured_at DESC LIMIT 1`, eventID).Scan(&exclusions, &report.Versions.Policy)
	if err == nil {
		report.Exclusions, err = reportExclusions(exclusions)
		if err != nil {
			return report, err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return report, err
	}
	err = source.pool.QueryRow(ctx, `SELECT solver_version, model_version
        FROM plan_versions WHERE event_id = $1 ORDER BY version DESC LIMIT 1`, eventID).Scan(&report.Versions.Solver, &report.Versions.Model)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return report, err
	}
	report.Provenance = []string{"SIMULATED"}
	return report, nil
}

func reportExclusions(contents []byte) (map[string]uint64, error) {
	var values []struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(contents, &values); err != nil {
		return nil, err
	}
	result := make(map[string]uint64)
	for _, value := range values {
		result[value.Reason]++
	}
	return result, nil
}

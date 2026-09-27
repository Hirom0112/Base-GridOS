package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func StorePublishedReport(ctx context.Context, pool *pgxpool.Pool, candidate report.EventReport, at time.Time) (report.EventReport, error) {
	if pool == nil || candidate.EventID == "" || candidate.PlanVersion == 0 || at.IsZero() {
		return report.EventReport{}, errors.New("versioned report, store, and production time required")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return report.EventReport{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var state, correlationID string
	var version int64
	if err = tx.QueryRow(ctx, "SELECT state, plan_version, correlation_id FROM dispatch_events WHERE event_id = $1 FOR UPDATE", candidate.EventID).Scan(&state, &version, &correlationID); err != nil {
		return report.EventReport{}, err
	}
	if version != int64(candidate.PlanVersion) {
		return report.EventReport{}, ErrEventState
	}
	if state == "REPORTED" {
		return scanPublishedReport(tx.QueryRow(ctx, "SELECT report, sha256 FROM event_reports WHERE event_id = $1 AND version = $2", candidate.EventID, version))
	}
	if state != "RECONCILED" || eventTransitions[state] != "REPORTED" {
		return report.EventReport{}, ErrEventState
	}
	encoded, err := json.Marshal(candidate)
	if err != nil {
		return report.EventReport{}, err
	}
	digest := sha256.Sum256(encoded)
	_, err = tx.Exec(ctx, "INSERT INTO event_reports (event_id, version, report, sha256, produced_at) VALUES ($1, $2, $3, $4, $5)", candidate.EventID, version, encoded, hex.EncodeToString(digest[:]), at)
	if err != nil {
		return report.EventReport{}, err
	}
	if _, err = tx.Exec(ctx, "UPDATE dispatch_events SET state = 'REPORTED', updated_at = $2 WHERE event_id = $1", candidate.EventID, at); err != nil {
		return report.EventReport{}, err
	}
	previous, err := json.Marshal(eventStateValue{State: "RECONCILED"})
	if err != nil {
		return report.EventReport{}, err
	}
	next, err := json.Marshal(eventStateValue{State: "REPORTED"})
	if err != nil {
		return report.EventReport{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_journal
		(occurred_at, actor_id, action, resource_type, resource_id, previous_values, new_values, correlation_id)
		VALUES ($1, 'workflow', 'EVENT_STATE_TRANSITIONED', 'dispatch_event', $2, $3, $4, $5)`, at, candidate.EventID, previous, next, correlationID)
	if err != nil {
		return report.EventReport{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return report.EventReport{}, err
	}
	return candidate, nil
}

func LoadPublishedReport(ctx context.Context, pool *pgxpool.Pool, eventID string) (*report.EventReport, error) {
	if pool == nil || eventID == "" {
		return nil, errors.New("report store and event identifier required")
	}
	value, err := scanPublishedReport(pool.QueryRow(ctx, `SELECT report, sha256 FROM event_reports
		WHERE event_id = $1 ORDER BY version DESC LIMIT 1`, eventID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if value.EventID != eventID {
		return nil, errors.New("published report identity mismatch")
	}
	return &value, nil
}

func LoadPublishedReportVersion(ctx context.Context, pool *pgxpool.Pool, eventID string, version uint64) (*report.EventReport, error) {
	if pool == nil || eventID == "" || version == 0 {
		return nil, errors.New("report store, event identifier, and positive version required")
	}
	value, err := scanPublishedReport(pool.QueryRow(ctx, `SELECT report, sha256 FROM event_reports
		WHERE event_id = $1 AND version = $2`, eventID, version))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if value.EventID != eventID || value.PlanVersion != version {
		return nil, errors.New("published report identity or version mismatch")
	}
	return &value, nil
}

func scanPublishedReport(row pgx.Row) (report.EventReport, error) {
	var encoded []byte
	var expected string
	if err := row.Scan(&encoded, &expected); err != nil {
		return report.EventReport{}, err
	}
	digest := sha256.Sum256(encoded)
	if hex.EncodeToString(digest[:]) != expected {
		return report.EventReport{}, errors.New("published report hash mismatch")
	}
	var value report.EventReport
	if err := json.Unmarshal(encoded, &value); err != nil {
		return report.EventReport{}, err
	}
	return value, nil
}

package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
)

func retentionStart(now time.Time) time.Time {
	year, month, day := now.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -6)
}

func partitionDay(at time.Time) time.Time {
	year, month, day := at.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func ensureTelemetryPartitions(ctx context.Context, tx pgx.Tx, rows []telemetryRow) error {
	seen := make(map[time.Time]struct{})
	for _, row := range rows {
		day := partitionDay(row.observation.GetObservationTime().AsTime())
		if _, exists := seen[day]; exists {
			continue
		}
		seen[day] = struct{}{}
		if err := ensureTelemetryPartition(ctx, tx, day); err != nil {
			return err
		}
	}
	return nil
}

func ensureTelemetryPartition(ctx context.Context, tx pgx.Tx, day time.Time) error {
	name := "telemetry_observations_" + day.Format("20060102")
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, name); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, name).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	var backfilled bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM telemetry_observations_default WHERE observed_at >= $1 AND observed_at < $2)`, day, day.AddDate(0, 0, 1)).Scan(&backfilled); err != nil {
		return err
	}
	if backfilled {
		if _, err := tx.Exec(ctx, "CREATE TABLE "+pgx.Identifier{name}.Sanitize()+" (LIKE telemetry_observations INCLUDING ALL)"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO "+pgx.Identifier{name}.Sanitize()+` SELECT * FROM telemetry_observations_default WHERE observed_at >= $1 AND observed_at < $2`, day, day.AddDate(0, 0, 1)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM telemetry_observations_default WHERE observed_at >= $1 AND observed_at < $2`, day, day.AddDate(0, 0, 1)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, fmt.Sprintf(`ALTER TABLE telemetry_observations ATTACH PARTITION %s FOR VALUES FROM ('%s') TO ('%s')`, pgx.Identifier{name}.Sanitize(), day.Format("2006-01-02 15:04:05-07"), day.AddDate(0, 0, 1).Format("2006-01-02 15:04:05-07")))
		return err
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s PARTITION OF telemetry_observations FOR VALUES FROM ('%s') TO ('%s')`, pgx.Identifier{name}.Sanitize(), day.Format("2006-01-02 15:04:05-07"), day.AddDate(0, 0, 1).Format("2006-01-02 15:04:05-07")))
	return err
}

func (store *TelemetryStore) Latest(ctx context.Context, deviceIDs []string, since time.Time) ([]*gridosv1.TelemetryObservation, error) {
	if len(deviceIDs) == 0 {
		return nil, nil
	}
	rows, err := store.pool.Query(ctx, `SELECT latest.payload
		FROM unnest($1::text[]) AS device(device_id)
		JOIN LATERAL (SELECT payload FROM telemetry_observations
			WHERE device_id = device.device_id AND observed_at >= $2
			ORDER BY observed_at DESC, sequence DESC LIMIT 1) AS latest ON true`, deviceIDs, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	latest := make([]*gridosv1.TelemetryObservation, 0, len(deviceIDs))
	for rows.Next() {
		var values []byte
		if err = rows.Scan(&values); err != nil {
			return nil, err
		}
		observation := &gridosv1.TelemetryObservation{}
		if err = protojson.Unmarshal(values, observation); err != nil {
			return nil, err
		}
		latest = append(latest, observation)
	}
	return latest, rows.Err()
}

func (store *TelemetryStore) Prune(ctx context.Context, now time.Time) (bool, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = ensureTelemetryPartition(ctx, tx, partitionDay(now)); err != nil {
		return false, err
	}
	if err = ensureTelemetryPartition(ctx, tx, partitionDay(now).AddDate(0, 0, 1)); err != nil {
		return false, err
	}
	rows, err := tx.Query(ctx, `SELECT child.relname FROM pg_inherits
		JOIN pg_class AS child ON child.oid = inhrelid
		WHERE inhparent = 'telemetry_observations'::regclass ORDER BY child.relname`)
	if err != nil {
		return false, err
	}
	oldest := ""
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return false, err
		}
		day, parseErr := time.Parse("20060102", strings.TrimPrefix(name, "telemetry_observations_"))
		if parseErr == nil && day.Before(retentionStart(now)) && (oldest == "" || name < oldest) {
			oldest = name
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return false, err
	}
	if oldest != "" {
		if _, err = tx.Exec(ctx, "DROP TABLE "+pgx.Identifier{oldest}.Sanitize()); err != nil {
			return false, err
		}
	}
	deleted, err := tx.Exec(ctx, `DELETE FROM telemetry_observations_default WHERE ctid IN (
		SELECT ctid FROM telemetry_observations_default WHERE observed_at < $1 LIMIT 10000)`, retentionStart(now))
	if err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return oldest != "" || deleted.RowsAffected() > 0, nil
}

package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
)

func TestPublishedReportVersionReadsExactHistory(t *testing.T) {
	pool := testDatabase(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	insertEvent(t, pool, "report-history", "REQUESTED")
	seedStoredPlan(t, pool, "report-history", at)
	_, err := pool.Exec(ctx, `INSERT INTO plan_versions
		(event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
		VALUES ('report-history', 2, 'stored-input', 'stored-eligibility', '{}', 'solver-1', 'model-1', 'history')`)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []report.EventReport{{EventID: "report-history", PlanVersion: 2, RequestedMW: 1}, {EventID: "report-history", PlanVersion: 3, RequestedMW: 2}} {
		encoded, encodeErr := json.Marshal(value)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		digest := sha256.Sum256(encoded)
		_, err = pool.Exec(ctx, `INSERT INTO event_reports(event_id, version, report, sha256, produced_at)
			VALUES ('report-history', $1, $2, $3, $4)`, value.PlanVersion, encoded, hex.EncodeToString(digest[:]), at)
		if err != nil {
			t.Fatal(err)
		}
	}
	old, err := LoadPublishedReportVersion(ctx, pool, "report-history", 2)
	if err != nil || old == nil || old.PlanVersion != 2 || old.RequestedMW != 1 {
		t.Fatalf("version 2 = %+v, error = %v", old, err)
	}
	latest, err := LoadPublishedReport(ctx, pool, "report-history")
	if err != nil || latest == nil || latest.PlanVersion != 3 {
		t.Fatalf("latest = %+v, error = %v", latest, err)
	}
	missing, err := LoadPublishedReportVersion(ctx, pool, "report-history", 4)
	if err != nil || missing != nil {
		t.Fatalf("unpublished history = %+v, error = %v", missing, err)
	}
}

func TestPublishedReportIsAtomicAndImmutable(t *testing.T) {
	pool := testDatabase(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	insertEvent(t, pool, "report-immutable", "REQUESTED")
	seedStoredPlan(t, pool, "report-immutable", at)
	if _, err := pool.Exec(ctx, "UPDATE dispatch_events SET state = 'RECONCILED' WHERE event_id = $1", "report-immutable"); err != nil {
		t.Fatal(err)
	}
	first := report.EventReport{EventID: "report-immutable", PlanVersion: 3, RequestedMW: 20, Versions: report.Versions{Policy: "policy-1"}}
	stored, err := StorePublishedReport(ctx, pool, first, at)
	if err != nil {
		t.Fatal(err)
	}
	if stored.RequestedMW != 20 {
		t.Fatalf("published report = %+v", stored)
	}
	first.RequestedMW = 99
	stored, err = StorePublishedReport(ctx, pool, first, at.Add(time.Minute))
	if err != nil || stored.RequestedMW != 20 {
		t.Fatalf("immutable retry = %+v, %v", stored, err)
	}
	var state, digest string
	var rows, audit int
	if err = pool.QueryRow(ctx, "SELECT state FROM dispatch_events WHERE event_id = $1", first.EventID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*), min(sha256) FROM event_reports WHERE event_id = $1", first.EventID).Scan(&rows, &digest); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM audit_journal WHERE resource_id = $1 AND action = 'EVENT_STATE_TRANSITIONED' AND new_values->>'state' = 'REPORTED'", first.EventID).Scan(&audit); err != nil {
		t.Fatal(err)
	}
	if state != "REPORTED" || rows != 1 || len(digest) != 64 || audit != 1 {
		t.Fatalf("atomic report state=%s rows=%d digest=%s audit=%d", state, rows, digest, audit)
	}
}

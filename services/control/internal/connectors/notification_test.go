package connectors

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNotificationDeliveryIsIdempotentAndRecorded(t *testing.T) {
	pool := notificationDatabase(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO member_anomaly_preferences
		(preference_id, member_id, opted_in, consent_text, consent_version, baseline_upper_kw,
		baseline_begin, baseline_end, effective_at, expires_at, correlation_id)
		VALUES ('pref-1', 'member-1', true, 'alert consent', 'v1', 2,
		'2026-08-01', '2026-09-01', '2026-08-01', '2026-09-01', 'corr-1')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO member_alerts
		(alert_id, member_id, kind, message, preference_id, evidence, observed_at, correlation_id)
		VALUES ('alert-1', 'member-1', 'ENERGY_ANOMALY_SIGNAL', 'energy anomaly signal',
		'pref-1', '{}', '2026-08-12', 'corr-1')`)
	if err != nil {
		t.Fatal(err)
	}
	connector := NewSimulatedNotification(pool)
	request := NotificationRequest{AlertID: "alert-1", IdempotencyKey: "delivery-1", CorrelationID: "corr-1"}
	first, err := connector.Deliver(ctx, request)
	if err != nil || first.AlertID != request.AlertID || first.Channel != "SIMULATED" || first.Outcome != "SIMULATED" {
		t.Fatalf("first delivery: %+v, %v", first, err)
	}
	second, err := connector.Deliver(ctx, request)
	if err != nil || second != first {
		t.Fatalf("idempotent retry: %+v, %v", second, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM member_alert_deliveries`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("delivery rows: %d, %v", count, err)
	}
	request.AlertID = "other-alert"
	if _, err := connector.Deliver(ctx, request); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed idempotency payload: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM member_alert_deliveries`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rows after rejected retry: %d, %v", count, err)
	}
}

func notificationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	adminURL := os.Getenv("GRIDOS_DATABASE_URL")
	if adminURL == "" {
		adminURL = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("gridos_notification_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})
	config, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	root := filepath.Join(os.Getenv("TEST_SRCDIR"), os.Getenv("TEST_WORKSPACE"))
	if os.Getenv("TEST_SRCDIR") == "" {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("test path unavailable")
		}
		root = filepath.Join(filepath.Dir(file), "../../../..")
	}
	migrations, err := filepath.Glob(filepath.Join(root, "database/migrations/*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(migrations)
	for _, path := range migrations {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := pool.Exec(ctx, string(contents)); execErr != nil {
			t.Fatalf("%s: %v", path, execErr)
		}
	}
	return pool
}

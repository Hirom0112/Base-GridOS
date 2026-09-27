package events

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/jackc/pgx/v5"
)

func TestTimelineSurfacesMalformedAuditValues(t *testing.T) {
	pool := exceptionDatabase(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `INSERT INTO audit_journal (occurred_at, actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1, 'operator', 'EVENT_STATE_TRANSITIONED', 'dispatch_event', 'event-wellformed', '{"state":"sent","reason":"published"}', 'audit'),
		($1, 'operator', 'EVENT_STATE_TRANSITIONED', 'dispatch_event', 'event-malformed', '{"state":7}', 'audit')`, pgx.QueryExecModeSimpleProtocol, at)
	if err != nil {
		t.Fatal(err)
	}
	source := NewPostgresSource(pool, nil, nil, func() time.Time { return at }, time.Minute)
	entries, err := source.Timeline(ctx, "event-wellformed")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_SENT || entries[0].GetReason() != "published" {
		t.Fatalf("well-formed timeline = %v", entries)
	}
	entries, err = source.Timeline(ctx, "event-malformed")
	if err == nil {
		t.Fatalf("malformed audit values decoded silently as %v", entries)
	}
}

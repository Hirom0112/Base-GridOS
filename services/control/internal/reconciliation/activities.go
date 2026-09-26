package reconciliation

import (
	"context"
	"errors"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUnsupportedBoundary = errors.New("delivery verification supports METER_NET_EXPORT and BATTERY_TERMINAL boundaries only")
	ErrMaxGapRequired      = errors.New("telemetry max gap is required")
)

type Input struct {
	EventID string
}

type Lifecycle interface {
	Advance(ctx context.Context, eventID, expected, next, actor string, at time.Time) (*gridosv1.DispatchEvent, error)
}

type Activities struct {
	Pool   *pgxpool.Pool
	Events Lifecycle
	Now    func() time.Time
	MaxGap time.Duration
}

func (activities *Activities) VerifyDelivery(ctx context.Context, input Input) error {
	if err := activities.verify(ctx, input.EventID); err != nil {
		return err
	}
	now := activities.Now()
	if _, err := activities.Events.Advance(ctx, input.EventID, "ACKNOWLEDGED_OR_UNCERTAIN", "EXECUTING", "reconciliation", now); err != nil {
		return err
	}
	_, err := activities.Events.Advance(ctx, input.EventID, "EXECUTING", "VERIFIED", "reconciliation", now)
	return err
}

func (activities *Activities) ReconcileLateMessages(ctx context.Context, input Input) error {
	if err := activities.verify(ctx, input.EventID); err != nil {
		return err
	}
	_, err := activities.Events.Advance(ctx, input.EventID, "VERIFIED", "RECONCILED", "reconciliation", activities.Now())
	return err
}

func (activities *Activities) verify(ctx context.Context, eventID string) error {
	if activities.MaxGap <= 0 {
		return ErrMaxGapRequired
	}
	stored, err := loadEvent(ctx, activities.Pool, eventID)
	if err != nil {
		return err
	}
	if _, supported := boundaryPower[stored.boundary]; !supported {
		return ErrUnsupportedBoundary
	}
	now := activities.Now()
	measurement := Measurement{Begin: stored.begin, End: earliest(stored.end, now), MaxGap: activities.MaxGap}
	event := NewEvent(measurement)
	commandIDs, err := loadCommands(ctx, activities.Pool, event, eventID)
	if err != nil {
		return err
	}
	if err = loadTelemetry(ctx, activities.Pool, event, stored.boundary); err != nil {
		return err
	}
	uncertain, err := loadUncertain(ctx, activities.Pool, commandIDs)
	if err != nil {
		return err
	}
	verification := event.Verify()
	return persist(ctx, activities.Pool, stored, verification, summarize(verification, uncertain), now)
}

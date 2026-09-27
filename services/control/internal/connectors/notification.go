package connectors

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const NotificationLiveSlot = "PENDING-LIVE"

type NotificationRequest struct {
	AlertID        string
	IdempotencyKey string
	CorrelationID  string
}

type NotificationDelivery struct {
	DeliveryID    string
	AlertID       string
	Channel       string
	AttemptedAt   time.Time
	Outcome       string
	CorrelationID string
}

type Notifications interface {
	Deliver(context.Context, NotificationRequest) (NotificationDelivery, error)
}

type SimulatedNotification struct {
	pool *pgxpool.Pool
}

func NewSimulatedNotification(pool *pgxpool.Pool) *SimulatedNotification {
	return &SimulatedNotification{pool: pool}
}

func (s *SimulatedNotification) Deliver(ctx context.Context, request NotificationRequest) (NotificationDelivery, error) {
	if request.AlertID == "" || request.IdempotencyKey == "" || request.CorrelationID == "" {
		return NotificationDelivery{}, ErrInvalidRequest
	}
	query := `SELECT delivery_id, alert_id, channel, attempted_at, outcome, correlation_id
		FROM member_alert_deliveries WHERE delivery_id = $1`
	var delivery NotificationDelivery
	err := s.pool.QueryRow(ctx, query, request.IdempotencyKey).Scan(
		&delivery.DeliveryID, &delivery.AlertID, &delivery.Channel,
		&delivery.AttemptedAt, &delivery.Outcome, &delivery.CorrelationID,
	)
	if err == nil {
		return matchNotification(request, delivery)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return NotificationDelivery{}, err
	}
	insert := `INSERT INTO member_alert_deliveries
		(delivery_id, alert_id, channel, attempted_at, outcome, correlation_id)
		VALUES ($1, $2, 'SIMULATED', clock_timestamp(), 'SIMULATED', $3)
		ON CONFLICT (delivery_id) DO NOTHING`
	if _, err := s.pool.Exec(ctx, insert, request.IdempotencyKey, request.AlertID, request.CorrelationID); err != nil {
		return NotificationDelivery{}, err
	}
	if err := s.pool.QueryRow(ctx, query, request.IdempotencyKey).Scan(
		&delivery.DeliveryID, &delivery.AlertID, &delivery.Channel,
		&delivery.AttemptedAt, &delivery.Outcome, &delivery.CorrelationID,
	); err != nil {
		return NotificationDelivery{}, err
	}
	return matchNotification(request, delivery)
}

func matchNotification(request NotificationRequest, delivery NotificationDelivery) (NotificationDelivery, error) {
	if delivery.AlertID != request.AlertID || delivery.CorrelationID != request.CorrelationID {
		return NotificationDelivery{}, ErrConflict
	}
	return delivery, nil
}

var _ Notifications = (*SimulatedNotification)(nil)

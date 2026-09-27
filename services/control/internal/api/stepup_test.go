package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/api/events"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/api/stepup"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type stepUpTestClaims struct {
	Subject     string    `json:"subject"`
	Action      string    `json:"action"`
	EventID     string    `json:"event_id"`
	PlanVersion uint64    `json:"plan_version"`
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Nonce       string    `json:"nonce"`
}

func TestStepUpApprovalRequiresBoundAssertionAndAuditsIt(t *testing.T) {
	pool := apiTestDatabase(t)
	seedAPIEvent(t, pool)
	key := []byte("local-step-up-test-key-32-bytes-long")
	t.Setenv("GRIDOS_STEP_UP_KEY", string(key))
	now := time.Now().UTC().Truncate(time.Second)
	service := NewService(NewPostgresEventStore(pool), fleet.NewTwin(time.Minute), nil, func() time.Time { return now })
	approval := connect.NewRequest(&gridosv1.ApproveEventRequest{EventId: "event-restart", PlanVersion: 3,
		IdempotencyKey: "approve-step-up", ApprovedBy: "forged-actor", ApprovedAt: timestamppb.New(now)})
	approval.Header().Set(roleHeader, "approver")
	approval.Header().Set("X-GridOS-Step-Up", signedStepUpTest(t, key, stepUpTestClaims{
		Subject: "approver-1", Action: "APPROVE_EVENT", EventID: "event-restart", PlanVersion: 3,
		IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), Nonce: "nonce-1",
	}))
	_, err := service.ApproveEvent(context.Background(), approval)
	require.NoError(t, err)
	missing := connect.NewRequest(&gridosv1.ApproveEventRequest{EventId: "event-restart", PlanVersion: 3,
		IdempotencyKey: "approve-missing-step-up", ApprovedBy: "forged-actor", ApprovedAt: timestamppb.New(now)})
	missing.Header().Set(roleHeader, "approver")
	_, err = service.ApproveEvent(context.Background(), missing)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	var actor string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT decided_by FROM operator_approvals WHERE event_id = 'event-restart'`).Scan(&actor))
	require.Equal(t, "approver-1", actor)
	var audits int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_journal WHERE action = 'STEP_UP_ACCEPTED' AND resource_id = 'event-restart' AND actor_id = 'approver-1'`).Scan(&audits))
	require.Equal(t, 1, audits)
}

func TestStepUpRejectsTamperingExpiryAndReplay(t *testing.T) {
	pool := apiTestDatabase(t)
	now := time.Date(2026, 9, 27, 2, 0, 0, 0, time.UTC)
	key := []byte("local-step-up-test-key-32-bytes-long")
	verifier := stepup.New(pool, key, func() time.Time { return now })
	claims := stepUpTestClaims{Subject: "approver-1", Action: "APPROVE_EVENT", EventID: "event-1", PlanVersion: 3,
		IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), Nonce: "nonce-replay"}
	ctx := context.Background()
	_, err := verifier.Verify(ctx, signedStepUpTest(t, []byte("wrong-step-up-test-key-32-bytes-long"), claims), "APPROVE_EVENT", "event-1", 3)
	require.Error(t, err)
	token := signedStepUpTest(t, key, claims)
	_, err = verifier.Verify(ctx, token, "EMERGENCY_STOP", "event-1", 3)
	require.Error(t, err)
	_, err = verifier.Verify(ctx, token, "APPROVE_EVENT", "event-2", 3)
	require.Error(t, err)
	claims.ExpiresAt = now.Add(-time.Second)
	_, err = verifier.Verify(ctx, signedStepUpTest(t, key, claims), "APPROVE_EVENT", "event-1", 3)
	require.Error(t, err)
	claims.ExpiresAt = now.Add(6 * time.Minute)
	_, err = verifier.Verify(ctx, signedStepUpTest(t, key, claims), "APPROVE_EVENT", "event-1", 3)
	require.Error(t, err)
	subject, err := verifier.Verify(ctx, token, "APPROVE_EVENT", "event-1", 3)
	require.NoError(t, err)
	require.Equal(t, "approver-1", subject)
	_, err = verifier.Verify(ctx, token, "APPROVE_EVENT", "event-1", 3)
	require.Error(t, err)
	var assertions, audits int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM step_up_assertions`).Scan(&assertions))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_journal WHERE action = 'STEP_UP_ACCEPTED'`).Scan(&audits))
	require.Equal(t, 1, assertions)
	require.Equal(t, 1, audits)
}

func TestStepUpEmergencyStopUsesAssertedSubject(t *testing.T) {
	pool := apiTestDatabase(t)
	seedAPIEvent(t, pool)
	_, err := pool.Exec(context.Background(), `UPDATE dispatch_events SET state = 'SENT' WHERE event_id = 'event-restart'`)
	require.NoError(t, err)
	key := []byte("local-step-up-test-key-32-bytes-long")
	t.Setenv("GRIDOS_STEP_UP_KEY", string(key))
	now := time.Now().UTC().Truncate(time.Second)
	controller := NewService(NewPostgresEventStore(pool), fleet.NewTwin(time.Minute), nil, func() time.Time { return now })
	controller.emergencyWorkflow = func(context.Context, string, dispatchWorkflowEmergencyStop) error { return nil }
	stopper := events.NewService(events.NewPostgresSource(pool, controller, nil, func() time.Time { return now }, time.Minute), time.Millisecond)
	stop := connect.NewRequest(&gridosv1.EmergencyStopRequest{EventId: "event-restart", IdempotencyKey: "stop-step-up",
		RequestedBy: "forged-actor", Reason: "safety", RequestedAt: timestamppb.New(now), CorrelationId: "stop-step-up"})
	stop.Header().Set(roleHeader, "operator")
	stop.Header().Set("X-GridOS-Step-Up", signedStepUpTest(t, key, stepUpTestClaims{Subject: "operator-1",
		Action: "EMERGENCY_STOP", EventID: "event-restart", IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), Nonce: "nonce-stop"}))
	response, err := stopper.EmergencyStop(context.Background(), stop)
	require.NoError(t, err)
	require.True(t, response.Msg.GetStopRequested())
	var actor string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT requested_by FROM emergency_stops WHERE idempotency_key = 'stop-step-up'`).Scan(&actor))
	require.Equal(t, "operator-1", actor)
	_, err = stopper.EmergencyStop(context.Background(), stop)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	missing := connect.NewRequest(&gridosv1.EmergencyStopRequest{EventId: "event-restart", IdempotencyKey: "stop-missing",
		RequestedBy: "operator-1", Reason: "safety", RequestedAt: timestamppb.New(now), CorrelationId: "stop-missing"})
	missing.Header().Set(roleHeader, "operator")
	_, err = stopper.EmergencyStop(context.Background(), missing)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func signedStepUpTest(t *testing.T, key []byte, claims stepUpTestClaims) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	mac := hmac.New(sha256.New, key)
	_, err = mac.Write(payload)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

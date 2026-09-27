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

func signedStepUpTest(t *testing.T, key []byte, claims stepUpTestClaims) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	mac := hmac.New(sha256.New, key)
	_, err = mac.Write(payload)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

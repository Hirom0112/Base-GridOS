package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/tests/integration/stepup"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestConsecutiveEventsUseNextDeviceGeneration(t *testing.T) {
	stack := startStack(t, "worker-termination")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.cohort(t), now, constantStateOfEnergy)
	firstID := fmt.Sprintf("generation-first-%d", now.UnixNano())
	stack.runEvent(t, ctx, firstID, now)
	var accepted int
	if err := stack.pool.QueryRow(ctx, `SELECT count(*) FROM command_intents intent
		JOIN command_acknowledgements ack USING (command_id)
		WHERE intent.event_id=$1 AND intent.setpoint_kw<>0 AND ack.receipt_status='ACCEPTED'`, firstID).Scan(&accepted); err != nil {
		t.Fatal(err)
	}
	if accepted == 0 {
		t.Fatal("first event has no accepted command")
	}
	secondNow := time.Now().UTC()
	stack.scenario.Event.StartAt = secondNow.Add(10 * time.Second)
	stack.scenario.Event.EndAt = secondNow.Add(70 * time.Second)
	stack.publishTelemetry(t, ctx, stack.cohort(t), secondNow, constantStateOfEnergy)
	secondID := fmt.Sprintf("generation-second-%d", secondNow.UnixNano())
	stack.createEvent(t, ctx, secondID)
	stack.waitEventState(t, ctx, secondID, "VALIDATED")
	stack.approveEvent(t, ctx, secondID, secondNow)
	stack.launchEvent(t, ctx, secondID, secondNow)
	stack.waitEventState(t, ctx, secondID, "ACKNOWLEDGED_OR_UNCERTAIN")
	var deviceID, firstState, secondState string
	var firstGeneration, secondGeneration int64
	err := stack.pool.QueryRow(ctx, `SELECT first.device_id, first.generation, second.generation, first_ack.receipt_status, second_ack.receipt_status
		FROM command_intents first JOIN command_intents second USING (device_id)
		JOIN command_acknowledgements first_ack ON first_ack.command_id=first.command_id
		JOIN command_acknowledgements second_ack ON second_ack.command_id=second.command_id
		WHERE first.event_id=$1 AND second.event_id=$2 AND first.setpoint_kw<>0 AND second.setpoint_kw<>0
		ORDER BY first.device_id LIMIT 1`, firstID, secondID).Scan(&deviceID, &firstGeneration, &secondGeneration, &firstState, &secondState)
	if err != nil {
		t.Fatal(err)
	}
	if firstState != "ACCEPTED" || secondState != "ACCEPTED" || secondGeneration <= firstGeneration {
		t.Fatalf("device %s consecutive receipts=%s/%s generations=%d/%d", deviceID, firstState, secondState, firstGeneration, secondGeneration)
	}
	stop := connect.NewRequest(&gridosv1.EmergencyStopRequest{
		EventId: secondID, IdempotencyKey: "stop-" + secondID, RequestedBy: "operator", Reason: "generation rehearsal",
		RequestedAt: timestamppb.Now(), CorrelationId: secondID,
	})
	stop.Header().Set("X-GridOS-Role", "operator")
	assertion, err := stepup.SignStepUpAssertion(stepup.LocalStepUpKey, "operator", "EMERGENCY_STOP", secondID, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	stop.Header().Set("X-GridOS-Step-Up", assertion)
	client := gridosv1connect.NewEventsServiceClient(stack.client, stack.controlURL)
	response, err := client.EmergencyStop(ctx, stop)
	if err != nil || !response.Msg.GetStopRequested() {
		t.Fatalf("emergency stop=%+v, %v", response, err)
	}
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		var generation int64
		var receipt string
		err = stack.pool.QueryRow(ctx, `SELECT intent.generation, ack.receipt_status FROM command_intents intent
			JOIN command_acknowledgements ack ON ack.command_id=intent.command_id
			WHERE intent.event_id=$1 AND intent.device_id=$2 AND intent.setpoint_kw=0 AND intent.command_id LIKE '%emergency%'
			ORDER BY intent.generation DESC LIMIT 1`, secondID, deviceID).Scan(&generation, &receipt)
		if err == nil {
			if generation <= secondGeneration || receipt != "ACCEPTED" {
				t.Fatalf("stop generation=%d receipt=%s after dispatch=%d", generation, receipt, secondGeneration)
			}
			t.Logf("device %s receipts %s/%s/%s generations %d/%d/%d", deviceID, firstState, secondState, receipt, firstGeneration, secondGeneration, generation)
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("emergency stop has no accepted gateway receipt")
}

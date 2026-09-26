package events

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type changingSource struct {
	mu       sync.RWMutex
	update   *gridosv1.EventUpdate
	timeline []*gridosv1.EventTimelineEntry
	stop     *gridosv1.EmergencyStopRequest
}

func (source *changingSource) Snapshot(context.Context, string) (*gridosv1.EventUpdate, error) {
	source.mu.RLock()
	defer source.mu.RUnlock()
	return proto.Clone(source.update).(*gridosv1.EventUpdate), nil
}

func (source *changingSource) Timeline(context.Context, string) ([]*gridosv1.EventTimelineEntry, error) {
	source.mu.RLock()
	defer source.mu.RUnlock()
	return source.timeline, nil
}

func (source *changingSource) RequestStop(_ context.Context, request *gridosv1.EmergencyStopRequest) (*gridosv1.EmergencyStopResponse, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.stop = request
	return &gridosv1.EmergencyStopResponse{
		EmergencyStop: &gridosv1.EmergencyStop{EmergencyStopId: "stop-1", EventId: request.GetEventId()},
		StopRequested: true,
	}, nil
}

func (source *changingSource) set(update *gridosv1.EventUpdate) {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.update = update
}

func TestWatchReportsSentOnlyAfterCommandsAreSent(t *testing.T) {
	source := &changingSource{update: eventUpdate(gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_APPROVED, 0, 0, 0)}
	service := NewService(source, 10*time.Millisecond)
	path, handler := gridosv1connect.NewEventsServiceHandler(service)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	client := gridosv1connect.NewEventsServiceClient(server.Client(), server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request := connect.NewRequest(&gridosv1.WatchEventRequest{EventId: "event-1"})
	request.Header().Set("X-GridOS-Role", "operator")
	stream, err := client.WatchEvent(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !stream.Receive() {
		t.Fatal(stream.Err())
	}
	if update := stream.Msg(); update.GetEvent().GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_APPROVED || update.GetFleet().GetSentMw() != 0 {
		t.Fatalf("initial update = %#v", update)
	}

	source.set(eventUpdate(gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_SENT, 5, 4, 3))
	if !stream.Receive() {
		t.Fatal(stream.Err())
	}
	update := stream.Msg()
	if update.GetEvent().GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_SENT || update.GetFleet().GetSentMw() != 5 || update.GetFleet().GetAcknowledgedMw() != 4 || update.GetFleet().GetDeliveredMw() != 3 {
		t.Fatalf("sent update = %#v", update)
	}
	if len(update.GetFleet().GetUncertaintyIntervals()) != 1 || len(update.GetH3()) != 1 || update.GetH3()[0].GetPower().GetSentMw() != 5 {
		t.Fatalf("H3 update = %#v", update.GetH3())
	}
}

func TestEmergencyStopReportsRequestedWithoutConfirmation(t *testing.T) {
	source := &changingSource{update: eventUpdate(gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_SENT, 5, 4, 3)}
	service := NewService(source, 10*time.Millisecond)
	request := connect.NewRequest(&gridosv1.EmergencyStopRequest{
		EventId: "event-1", IdempotencyKey: "stop-key", RequestedBy: "operator-1", Reason: "operator request",
		RequestedAt: timestamppb.New(time.Unix(100, 0)), CorrelationId: "correlation-1",
	})
	request.Header().Set("X-GridOS-Role", "operator")
	response, err := service.EmergencyStop(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !response.Msg.GetStopRequested() || response.Msg.GetEmergencyStop().GetEmergencyStopId() != "stop-1" {
		t.Fatalf("stop response = %#v", response.Msg)
	}
	if source.stop.GetIdempotencyKey() != "stop-key" {
		t.Fatalf("recorded stop = %#v", source.stop)
	}
}

func TestGetEventTimelineReturnsOrderedAuditDecisions(t *testing.T) {
	source := &changingSource{timeline: []*gridosv1.EventTimelineEntry{
		{Sequence: 1, Action: "EVENT_STATE_TRANSITIONED", State: gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_SENT},
		{Sequence: 2, Action: "COMMAND_RETRIED", Reason: "gateway unavailable"},
		{Sequence: 3, Action: "EVENT_RECOVERED", Reason: "worker resumed"},
	}}
	service := NewService(source, 10*time.Millisecond)
	request := connect.NewRequest(&gridosv1.GetEventTimelineRequest{EventId: "event-1"})
	request.Header().Set("X-GridOS-Role", "operator")
	response, err := service.GetEventTimeline(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Msg.GetEntries()) != 3 || response.Msg.GetEntries()[0].GetSequence() != 1 || response.Msg.GetEntries()[2].GetReason() != "worker resumed" {
		t.Fatalf("timeline = %#v", response.Msg.GetEntries())
	}
}

func eventUpdate(state gridosv1.DispatchEventState, sentMW, acknowledgedMW, deliveredMW float64) *gridosv1.EventUpdate {
	power := &gridosv1.EventPowerAggregate{
		SentMw: sentMW, AcknowledgedMw: acknowledgedMW, DeliveredMw: deliveredMW,
		UncertaintyIntervals: []*gridosv1.UncertaintyInterval{{DeviceId: "device-1"}},
	}
	return &gridosv1.EventUpdate{
		Event: &gridosv1.DispatchEvent{EventId: "event-1", State: state},
		Fleet: power,
		H3: []*gridosv1.H3EventPowerAggregate{{
			H3Cell: "cell-1",
			Power:  proto.Clone(power).(*gridosv1.EventPowerAggregate),
		}},
	}
}

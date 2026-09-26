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
)

type changingSource struct {
	mu     sync.RWMutex
	update *gridosv1.EventUpdate
}

func (source *changingSource) Snapshot(context.Context, string) (*gridosv1.EventUpdate, error) {
	source.mu.RLock()
	defer source.mu.RUnlock()
	return proto.Clone(source.update).(*gridosv1.EventUpdate), nil
}

func (source *changingSource) Timeline(context.Context, string) ([]*gridosv1.EventTimelineEntry, error) {
	return nil, nil
}

func (source *changingSource) RequestStop(context.Context, *gridosv1.EmergencyStopRequest) (*gridosv1.EmergencyStopResponse, error) {
	return nil, nil
}

func (source *changingSource) set(update *gridosv1.EventUpdate) {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.update = update
}

func TestWatchReportsSentOnlyAfterCommandsAreSent(t *testing.T) {
	source := &changingSource{update: eventUpdate(gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_APPROVED, 0)}
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

	source.set(eventUpdate(gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_SENT, 5))
	if !stream.Receive() {
		t.Fatal(stream.Err())
	}
	update := stream.Msg()
	if update.GetEvent().GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_SENT || update.GetFleet().GetSentMw() != 5 {
		t.Fatalf("sent update = %#v", update)
	}
	if len(update.GetH3()) != 1 || update.GetH3()[0].GetPower().GetSentMw() != 5 {
		t.Fatalf("H3 update = %#v", update.GetH3())
	}
}

func eventUpdate(state gridosv1.DispatchEventState, sentMW float64) *gridosv1.EventUpdate {
	return &gridosv1.EventUpdate{
		Event: &gridosv1.DispatchEvent{EventId: "event-1", State: state},
		Fleet: &gridosv1.EventPowerAggregate{SentMw: sentMW},
		H3: []*gridosv1.H3EventPowerAggregate{{
			H3Cell: "cell-1",
			Power:  &gridosv1.EventPowerAggregate{SentMw: sentMW},
		}},
	}
}

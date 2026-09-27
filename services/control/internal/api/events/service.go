package events

import (
	"context"
	"errors"
	"slices"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/api/stepup"
	"google.golang.org/protobuf/proto"
)

type Source interface {
	Snapshot(context.Context, string) (*gridosv1.WatchEventResponse, error)
	Timeline(context.Context, string) ([]*gridosv1.EventTimelineEntry, error)
	TimelineExceptions(context.Context, string) ([]*gridosv1.EventException, error)
	Commands(context.Context, string) (*gridosv1.ListEventCommandsResponse, error)
	RequestStop(context.Context, *gridosv1.EmergencyStopRequest) (*gridosv1.EmergencyStopResponse, error)
}

type Service struct {
	source       Source
	pollInterval time.Duration
	stepUp       *stepup.Verifier
}

func NewService(source Source, pollInterval time.Duration) *Service {
	service := &Service{source: source, pollInterval: pollInterval}
	if postgres, ok := source.(*PostgresSource); ok {
		service.stepUp = stepup.FromEnvironment(postgres.pool, time.Now)
	} else {
		service.stepUp = stepup.FromEnvironment(nil, time.Now)
	}
	return service
}

func (service *Service) GetEventTimeline(ctx context.Context, request *connect.Request[gridosv1.GetEventTimelineRequest]) (*connect.Response[gridosv1.GetEventTimelineResponse], error) {
	if err := authorize(request.Header().Get("X-GridOS-Role"), "operator", "approver", "analyst", "service"); err != nil {
		return nil, err
	}
	if request.Msg.GetEventId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event identifier is required"))
	}
	entries, err := service.source.Timeline(ctx, request.Msg.GetEventId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	exceptions, err := service.source.TimelineExceptions(ctx, request.Msg.GetEventId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&gridosv1.GetEventTimelineResponse{Entries: entries, Exceptions: exceptions}), nil
}

func (service *Service) EmergencyStop(ctx context.Context, request *connect.Request[gridosv1.EmergencyStopRequest]) (*connect.Response[gridosv1.EmergencyStopResponse], error) {
	if err := authorize(request.Header().Get("X-GridOS-Role"), "operator", "approver"); err != nil {
		return nil, err
	}
	stop := proto.Clone(request.Msg).(*gridosv1.EmergencyStopRequest)
	if stop.GetEventId() == "" || stop.GetIdempotencyKey() == "" || stop.GetReason() == "" || stop.GetRequestedAt() == nil || !stop.GetRequestedAt().IsValid() || stop.GetCorrelationId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("complete emergency stop is required"))
	}
	if service.stepUp != nil {
		subject, err := service.stepUp.Verify(ctx, request.Header().Get("X-GridOS-Step-Up"), "EMERGENCY_STOP", stop.GetEventId(), 0)
		if err != nil {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
		stop.RequestedBy = subject
	}
	if stop.GetRequestedBy() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("requester required"))
	}
	response, err := service.source.RequestStop(ctx, stop)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(response), nil
}

func (service *Service) WatchEvent(ctx context.Context, request *connect.Request[gridosv1.WatchEventRequest], stream *connect.ServerStream[gridosv1.WatchEventResponse]) error {
	if err := authorize(request.Header().Get("X-GridOS-Role"), "operator", "approver", "analyst", "service"); err != nil {
		return err
	}
	if request.Msg.GetEventId() == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("event identifier is required"))
	}
	ticker := time.NewTicker(service.pollInterval)
	defer ticker.Stop()
	var previous *gridosv1.WatchEventResponse
	for {
		update, err := service.source.Snapshot(ctx, request.Msg.GetEventId())
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if !proto.Equal(previous, update) {
			if err = stream.Send(update); err != nil {
				return err
			}
			previous = proto.Clone(update).(*gridosv1.WatchEventResponse)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func authorize(role string, allowed ...string) error {
	if role == "" {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("role required"))
	}
	if !slices.Contains(allowed, role) {
		return connect.NewError(connect.CodePermissionDenied, errors.New("role is not authorized"))
	}
	return nil
}

package ingest

import (
	"context"
	"errors"
	"slices"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Store interface {
	Write(context.Context, []*gridosv1.TelemetryObservation) ([]*gridosv1.TelemetryObservation, error)
}

type Twin interface {
	Accept(*gridosv1.TelemetryObservation)
}

type Service struct {
	store Store
	twin  Twin
	now   func() time.Time
}

func NewService(store Store, twin Twin, now func() time.Time) *Service {
	return &Service{store: store, twin: twin, now: now}
}

func (service *Service) PublishTelemetry(ctx context.Context, request *connect.Request[gridosv1.PublishTelemetryRequest]) (*connect.Response[gridosv1.PublishTelemetryResponse], error) {
	if err := validate(request.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ordered := slices.Clone(request.Msg.GetObservations())
	slices.SortFunc(ordered, compareObservations)
	inserted, err := service.store.Write(ctx, ordered)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	slices.SortFunc(inserted, compareObservations)
	for _, observation := range inserted {
		service.twin.Accept(observation)
	}
	receivedAt := service.now()
	return connect.NewResponse(&gridosv1.PublishTelemetryResponse{
		DurableReceiptId:  request.Msg.GetGatewayId() + ":" + receivedAt.UTC().Format(time.RFC3339Nano),
		ObservationIds:    uniqueObservationIDs(ordered),
		DurablyReceivedAt: timestamppb.New(receivedAt),
	}), nil
}

func validate(request *gridosv1.PublishTelemetryRequest) error {
	if request.GetGatewayId() == "" {
		return errors.New("gateway identifier is required")
	}
	for _, observation := range request.GetObservations() {
		if observation.GetObservationId() == "" || observation.GetDeviceId() == "" || observation.GetSequence() == 0 {
			return errors.New("observation identifier, device identifier, and sequence are required")
		}
		if observation.GetObservationTime() == nil || !observation.GetObservationTime().IsValid() {
			return errors.New("valid observation time is required")
		}
	}
	return nil
}

func compareObservations(left, right *gridosv1.TelemetryObservation) int {
	if compared := compare(left.GetDeviceId(), right.GetDeviceId()); compared != 0 {
		return compared
	}
	return compare(left.GetSequence(), right.GetSequence())
}

func compare[T ~string | ~uint64](left, right T) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func uniqueObservationIDs(observations []*gridosv1.TelemetryObservation) []string {
	ids := make([]string, 0, len(observations))
	var previous sequenceKey
	for index, observation := range observations {
		key := sequenceKey{deviceID: observation.GetDeviceId(), sequence: observation.GetSequence()}
		if index > 0 && key == previous {
			continue
		}
		ids = append(ids, observation.GetObservationId())
		previous = key
	}
	return ids
}

type sequenceKey struct {
	deviceID string
	sequence uint64
}

package ingest

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type recordingStore struct {
	entered chan struct{}
	release chan struct{}
	stored  map[observationKey]*gridosv1.TelemetryObservation
}

type observationKey struct {
	deviceID string
	sequence uint64
}

func (store *recordingStore) Write(ctx context.Context, _ string, observations []*gridosv1.TelemetryObservation) ([]*gridosv1.TelemetryObservation, error) {
	if store.entered != nil {
		close(store.entered)
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-store.release:
	}
	inserted := make([]*gridosv1.TelemetryObservation, 0, len(observations))
	for _, observation := range observations {
		key := observationKey{deviceID: observation.GetDeviceId(), sequence: observation.GetSequence()}
		if _, exists := store.stored[key]; exists {
			continue
		}
		store.stored[key] = observation
		inserted = append(inserted, observation)
	}
	slices.SortFunc(inserted, func(left, right *gridosv1.TelemetryObservation) int {
		return int(left.GetSequence()) - int(right.GetSequence())
	})
	return inserted, nil
}

type recordingTwin struct {
	observations []*gridosv1.TelemetryObservation
}

func (twin *recordingTwin) Accept(observation *gridosv1.TelemetryObservation) {
	twin.observations = append(twin.observations, observation)
}

func TestPublishTelemetryAcknowledgesOnlyAfterDurableWrite(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{})
	store := &recordingStore{entered: entered, release: release, stored: make(map[observationKey]*gridosv1.TelemetryObservation)}
	service := NewService(store, &recordingTwin{}, func() time.Time { return time.Unix(100, 0) })
	done := make(chan error, 1)
	go func() {
		_, err := service.PublishTelemetry(context.Background(), connect.NewRequest(&gridosv1.PublishTelemetryRequest{
			GatewayId:    "gateway-1",
			Observations: []*gridosv1.TelemetryObservation{observation(1, gridosv1.ValueState_VALUE_STATE_PRESENT)},
		}))
		done <- err
	}()

	<-entered
	select {
	case <-done:
		t.Fatal("receipt returned before durable write")
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPublishTelemetryOrdersSequencesAndDropsDuplicates(t *testing.T) {
	release := make(chan struct{})
	close(release)
	store := &recordingStore{release: release, stored: make(map[observationKey]*gridosv1.TelemetryObservation)}
	twin := &recordingTwin{}
	service := NewService(store, twin, time.Now)
	request := connect.NewRequest(&gridosv1.PublishTelemetryRequest{GatewayId: "gateway-1", Observations: []*gridosv1.TelemetryObservation{
		observation(2, gridosv1.ValueState_VALUE_STATE_STALE),
		observation(1, gridosv1.ValueState_VALUE_STATE_MISSING),
		observation(2, gridosv1.ValueState_VALUE_STATE_PRESENT),
	}})
	response, err := service.PublishTelemetry(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(response.Msg.GetObservationIds(), []string{"device-1-1", "device-1-2"}) {
		t.Fatalf("receipt observations = %v", response.Msg.GetObservationIds())
	}
	if len(twin.observations) != 2 || twin.observations[0].GetSequence() != 1 || twin.observations[1].GetSequence() != 2 {
		t.Fatalf("twin observations = %v", twin.observations)
	}
	if twin.observations[0].GetValueState() != gridosv1.ValueState_VALUE_STATE_MISSING || twin.observations[1].GetValueState() != gridosv1.ValueState_VALUE_STATE_STALE {
		t.Fatalf("value states = %v, %v", twin.observations[0].GetValueState(), twin.observations[1].GetValueState())
	}
}

func TestPublishTelemetryDoesNotAcknowledgeFailedWrite(t *testing.T) {
	service := NewService(failingStore{}, &recordingTwin{}, time.Now)
	response, err := service.PublishTelemetry(context.Background(), connect.NewRequest(&gridosv1.PublishTelemetryRequest{
		GatewayId:    "gateway-1",
		Observations: []*gridosv1.TelemetryObservation{observation(1, gridosv1.ValueState_VALUE_STATE_PRESENT)},
	}))
	if err == nil || response != nil {
		t.Fatalf("response, error = %v, %v; want nil response and error", response, err)
	}
}

func TestPublishTelemetryReturnsPermanentRejectionForExpiredObservation(t *testing.T) {
	request := connect.NewRequest(&gridosv1.PublishTelemetryRequest{GatewayId: "gateway-1", Observations: []*gridosv1.TelemetryObservation{observation(1, gridosv1.ValueState_VALUE_STATE_PRESENT)}})
	response, err := NewService(expiredStore{}, &recordingTwin{}, time.Now).PublishTelemetry(context.Background(), request)
	if response != nil || connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expired response, code = %v, %v; want nil, invalid argument", response, connect.CodeOf(err))
	}
	_, err = NewService(failingStore{}, &recordingTwin{}, time.Now).PublishTelemetry(context.Background(), request)
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("failed write code = %v; want internal", connect.CodeOf(err))
	}
}

type expiredStore struct{}

func (expiredStore) Write(context.Context, string, []*gridosv1.TelemetryObservation) ([]*gridosv1.TelemetryObservation, error) {
	return nil, storage.ErrTelemetryExpired
}

type failingStore struct{}

func (failingStore) Write(context.Context, string, []*gridosv1.TelemetryObservation) ([]*gridosv1.TelemetryObservation, error) {
	return nil, errors.New("write failed")
}

func observation(sequence uint64, state gridosv1.ValueState) *gridosv1.TelemetryObservation {
	deviceID := "device-1"
	return &gridosv1.TelemetryObservation{
		ObservationId:   deviceID + "-" + strconv.FormatUint(sequence, 10),
		DeviceId:        deviceID,
		Sequence:        sequence,
		ObservationTime: timestamppb.New(time.Unix(int64(sequence), 0)),
		ValueState:      state,
	}
}

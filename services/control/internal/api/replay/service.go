package replay

import (
	"context"
	"errors"
	"os"
	"slices"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	corereplay "github.com/Hirom0112/Base-GridOS/services/control/internal/replay"
)

type TimelineSource interface {
	Timeline(context.Context, string) ([]*gridosv1.EventTimelineEntry, error)
}

type Service struct {
	directory string
	source    corereplay.Source
	timeline  TimelineSource
	planner   corereplay.Planner
}

func NewService(directory string, source corereplay.Source, timeline TimelineSource, planner corereplay.Planner) *Service {
	return &Service{directory: directory, source: source, timeline: timeline, planner: planner}
}

func (service *Service) ReplayEvent(ctx context.Context, request *connect.Request[gridosv1.ReplayEventRequest]) (*connect.Response[gridosv1.ReplayEventResponse], error) {
	role := request.Header().Get("X-GridOS-Role")
	if role == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("role required"))
	}
	if !slices.Contains([]string{"operator", "approver", "analyst", "service"}, role) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("role is not authorized"))
	}
	if request.Msg.GetEventId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event identifier is required"))
	}
	manifest, err := corereplay.Load(service.directory, request.Msg.GetEventId())
	if os.IsNotExist(err) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	result, err := corereplay.Run(ctx, manifest, service.source, service.planner)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	updates, err := service.timeline.Timeline(ctx, manifest.EventID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	differences := make([]*gridosv1.ReplayDifference, 0, len(result.Differences))
	for _, difference := range result.Differences {
		differences = append(differences, &gridosv1.ReplayDifference{Field: difference.Field, ExpectedJson: string(difference.Expected), ActualJson: string(difference.Actual)})
	}
	return connect.NewResponse(&gridosv1.ReplayEventResponse{
		Seed: manifest.Seed, InputSnapshotId: manifest.InputSnapshotID, EligibilitySnapshotId: manifest.EligibilitySnapshotID,
		PolicyVersion: manifest.PolicyVersion, SolverVersion: manifest.SolverVersion, FallbackVersion: manifest.FallbackVersion,
		CodeVersion: manifest.CodeVersion, FleetSha256: manifest.FleetSHA256, ScenarioSha256: manifest.ScenarioSHA256,
		Updates: updates, DiffStatus: result.Status, Differences: differences,
	}), nil
}

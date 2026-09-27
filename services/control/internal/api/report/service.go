package report

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	core "github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	source core.Source
}

func NewService(source core.Source) *Service {
	return &Service{source: source}
}

func (service *Service) GetEventReport(ctx context.Context, request *connect.Request[gridosv1.GetEventReportRequest]) (*connect.Response[gridosv1.GetEventReportResponse], error) {
	role := request.Header().Get("X-GridOS-Role")
	if role == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("role required"))
	}
	switch role {
	case "partner":
		if !request.Msg.GetPartnerView() {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("partner view required"))
		}
	case "operator", "approver", "analyst", "service":
	default:
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("role is not authorized"))
	}
	if request.Msg.GetEventId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event identifier required"))
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	value, err := core.Build(ctx, service.source, request.Msg.GetEventId())
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var encoded []byte
	if request.Msg.GetPartnerView() {
		encoded, err = json.Marshal(core.ForPartner(value))
	} else {
		encoded, err = json.Marshal(value)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&gridosv1.GetEventReportResponse{ReportJson: string(encoded)}), nil
}

func (service *Service) CompareEventReports(ctx context.Context, request *connect.Request[gridosv1.CompareEventReportsRequest]) (*connect.Response[gridosv1.CompareEventReportsResponse], error) {
	role := request.Header().Get("X-GridOS-Role")
	if role == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("role required"))
	}
	switch role {
	case "operator", "approver", "analyst", "service":
	default:
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("role is not authorized"))
	}
	if request.Msg.GetEventIdA() == "" || request.Msg.GetEventIdB() == "" || request.Msg.PlanVersionA != nil && request.Msg.GetPlanVersionA() == 0 || request.Msg.PlanVersionB != nil && request.Msg.GetPlanVersionB() == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event identifiers and positive requested versions required"))
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	first, err := core.BuildPublished(ctx, service.source, request.Msg.GetEventIdA(), request.Msg.PlanVersionA)
	if errors.Is(err, core.ErrNotPublished) || errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	second, err := core.BuildPublished(ctx, service.source, request.Msg.GetEventIdB(), request.Msg.PlanVersionB)
	if errors.Is(err, core.ErrNotPublished) || errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &gridosv1.CompareEventReportsResponse{}
	for _, difference := range core.Compare(first, second) {
		response.Differences = append(response.Differences, &gridosv1.EventReportDifference{Field: difference.Field, Before: difference.Before, After: difference.After})
	}
	return connect.NewResponse(response), nil
}

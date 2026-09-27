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

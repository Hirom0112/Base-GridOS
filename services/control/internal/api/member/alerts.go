package member

import (
	"context"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) ListHomeActivityAlerts(ctx context.Context, request *connect.Request[gridosv1.ListHomeActivityAlertsRequest]) (*connect.Response[gridosv1.ListHomeActivityAlertsResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	memberID := request.Msg.GetMemberId()
	if err := service.authorize(ctx, request.Header().Get("X-GridOS-Role"), request.Header().Get("X-GridOS-Member-ID"), memberID, "", false); err != nil {
		return nil, err
	}
	rows, err := service.pool.Query(ctx, `SELECT alert_id,member_id,message,observed_at,evidence->>'consent_version'
		FROM member_alerts WHERE member_id = $1 ORDER BY observed_at DESC, alert_id DESC LIMIT 50`, memberID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()
	response := &gridosv1.ListHomeActivityAlertsResponse{}
	for rows.Next() {
		var alert gridosv1.HomeActivityAlert
		var observedAt time.Time
		if err := rows.Scan(&alert.AlertId, &alert.MemberId, &alert.Description, &observedAt, &alert.ConsentVersion); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		alert.ObservedAt = timestamppb.New(observedAt)
		response.Alerts = append(response.Alerts, &alert)
	}
	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(response), nil
}

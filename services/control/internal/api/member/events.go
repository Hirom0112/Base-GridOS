package member

import (
	"context"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) recentEvents(ctx context.Context, siteID string) ([]*gridosv1.GridEvent, error) {
	devices := make([]string, 0, len(service.sites[siteID].GetDevices()))
	for _, device := range service.sites[siteID].GetDevices() {
		devices = append(devices, device.GetDeviceId())
	}
	if len(devices) == 0 {
		return nil, nil
	}
	rows, err := service.pool.Query(ctx, `SELECT event.event_id, request.begin_time, request.end_time,
		request.event_type, version.plan
		FROM dispatch_events event JOIN dispatch_requests request ON request.request_id = event.request_id
		JOIN plan_versions version ON version.event_id = event.event_id AND version.version = event.plan_version
		WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(version.plan->'deviceSchedules', '[]'::jsonb)) schedule
			WHERE schedule->>'deviceId' = ANY($1::text[]))
		OR EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(version.plan->'exclusions', '[]'::jsonb)) exclusion
			WHERE exclusion->>'deviceId' = ANY($1::text[]))
		ORDER BY request.begin_time DESC, event.event_id DESC LIMIT 10`, devices)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []*gridosv1.GridEvent
	for rows.Next() {
		var id, eventType string
		var begin, end time.Time
		var encoded []byte
		if err := rows.Scan(&id, &begin, &end, &eventType, &encoded); err != nil {
			return nil, err
		}
		var plan gridosv1.DispatchPlan
		if err := protojson.Unmarshal(encoded, &plan); err != nil {
			return nil, err
		}
		event := &gridosv1.GridEvent{EventId: id, BeginTime: timestamppb.New(begin),
			EndTime: timestamppb.New(end), EventType: eventType}
		for _, schedule := range plan.GetDeviceSchedules() {
			for _, deviceID := range devices {
				if schedule.GetDeviceId() == deviceID {
					event.Participated = true
				}
			}
		}
		for _, exclusion := range plan.GetExclusions() {
			for _, deviceID := range devices {
				if exclusion.GetDeviceId() == deviceID {
					event.ExclusionReason = exclusion.GetReason()
					event.ParticipationExplanation = exclusion.GetDetail()
				}
			}
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

package api

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
)

func TestTelemetryMetricsReadsDurableLatestObservations(t *testing.T) {
	ctx := context.Background()
	pool := apiTestDatabase(t)
	now := time.Now().UTC()
	sites := []*gridosv1.AuthorizedSite{{
		Site:    &gridosv1.Site{SiteId: "site-metrics"},
		Devices: []*gridosv1.Device{{DeviceId: "device-fresh"}, {DeviceId: "device-stale"}, {DeviceId: "device-unseen"}},
	}}
	for _, row := range []struct {
		device string
		at     time.Time
	}{
		{"device-fresh", now.Add(-5 * time.Second)},
		{"device-stale", now.Add(-time.Minute)},
	} {
		_, err := pool.Exec(ctx, `INSERT INTO telemetry_observations (observed_at, device_id, sequence, observation_id, payload)
			VALUES ($1, $2, 1, $2, '{}')`, row.at, row.device)
		if err != nil {
			t.Fatal(err)
		}
	}
	snapshotter := NewDurableFleetSnapshotter(pool, nil, nil, sites, func() time.Time { return now })
	total, stale, freshness, err := snapshotter.TelemetryMetrics(ctx)
	if err != nil || total != 3 || stale != 2 || freshness != 5*time.Second {
		t.Fatalf("total=%d stale=%d freshness=%s err=%v", total, stale, freshness, err)
	}
}

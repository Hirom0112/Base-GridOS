package reconciliation

import (
	"context"
	"testing"
	"time"
)

func TestVerifyDeliveryExtendsPartialIntervalEnd(t *testing.T) {
	harness := newActivityHarness(t)
	for _, check := range []struct {
		now time.Time
		end time.Time
	}{
		{harness.begin.Add(30 * time.Second), harness.begin.Add(30 * time.Second)},
		{harness.begin.Add(10 * time.Minute), harness.begin.Add(5 * time.Minute)},
	} {
		harness.now = check.now
		if err := harness.activities.VerifyDelivery(context.Background(), Input{EventID: "event-1"}); err != nil {
			t.Fatal(err)
		}
		var end time.Time
		var requestedKWh float64
		err := harness.pool.QueryRow(context.Background(), `SELECT interval_end_time,
			requested_kw * extract(epoch FROM interval_end_time - interval_begin_time) / 3600
			FROM verification_summaries WHERE event_id = 'event-1' AND interval_begin_time = $1`, harness.begin).Scan(&end, &requestedKWh)
		if err != nil {
			t.Fatal(err)
		}
		want := 7 * check.end.Sub(harness.begin).Hours()
		if !end.Equal(check.end) || requestedKWh < want-1e-9 || requestedKWh > want+1e-9 {
			t.Fatalf("at %s interval end = %s requested kWh = %f, want %s and %f", check.now.Sub(harness.begin), end.Sub(harness.begin), requestedKWh, check.end.Sub(harness.begin), want)
		}
	}
}

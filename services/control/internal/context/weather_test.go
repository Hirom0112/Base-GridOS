package context

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWeatherAlertUsesFixtureProvenance(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	directory := t.TempDir()
	alertPath := filepath.Join(directory, "austin_alerts.json")
	alert := fmt.Sprintf(`{"features":[{"id":"travis-test","properties":{"areaDesc":"Travis County","geocode":{"UGC":["TXZ192"],"SAME":["048453"]},"sent":%q,"effective":%q,"expires":%q,"event":"Heat Warning","severity":"Severe"}}]}`,
		now.Add(-time.Minute).Format(time.RFC3339), now.Add(-time.Minute).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339))
	if err := os.WriteFile(alertPath, []byte(alert), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, provenance := range []string{"SIMULATED", "CONFIRMED_PUBLIC"} {
		if err := os.WriteFile(filepath.Join(directory, "PROVENANCE.md"), []byte("Provenance: "+provenance+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		alerts, err := loadAlerts(alertPath, "austin", now)
		if err != nil || len(alerts) != 1 || alerts[0].Source.Provenance != provenance {
			t.Fatalf("fixture %s alerts=%+v error=%v", provenance, alerts, err)
		}
	}
}

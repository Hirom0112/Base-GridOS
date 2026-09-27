package load

import (
	"testing"
	"time"
)

func TestDispatchPath(t *testing.T) {
	if testing.Short() {
		t.Skip("isolated dispatch load run")
	}
	result := runDispatchPath(t, 5000)
	if result.persisted != 5000 || result.sent == 0 || result.acknowledged == 0 || result.sentBeforePersisted != 0 || result.readSamples < 100 || result.readP95 >= 500*time.Millisecond {
		t.Fatalf("persisted=%d sent=%d acknowledged=%d sent_before_persisted=%d read_samples=%d read_p95=%s", result.persisted, result.sent, result.acknowledged, result.sentBeforePersisted, result.readSamples, result.readP95)
	}
	t.Logf("persisted=%d sent=%d acknowledged=%d sent_before_persisted=%d read_samples=%d read_p95=%s", result.persisted, result.sent, result.acknowledged, result.sentBeforePersisted, result.readSamples, result.readP95)
}

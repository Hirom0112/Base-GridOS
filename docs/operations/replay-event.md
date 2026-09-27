# Replay an event

Use replay to investigate a completed event or compare a candidate decision with what was approved. Replay reads the recorded seed, input and eligibility snapshot IDs, plan version, policies, and code versions. It must not send gateway commands or alter the original event.

1. Open the event's evidence view and choose **Replay event**. Keep the original event ID visible while reviewing the replay manifest, timeline, and plan difference.
2. Check source provenance and freshness for every input. A replay with different consent, policy, or frozen input is a different scenario and must be labelled as such.
3. If the replay disagrees, retain both manifests and the audit chain for investigation. Do not rewrite the approved plan or report.

The isolated API check returns the manifest, timeline, and deterministic difference:

```sh
go test ./services/control/internal/api/replay -run '^TestReplayEventReturnsManifestTimelineAndDiff$' -count=1
```

Observed output: `ok github.com/Hirom0112/Base-GridOS/services/control/internal/api/replay 0.301s`.

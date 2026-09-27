# Replay an event

Use this when a report, plan, or command outcome needs investigation. Replay
reads the recorded manifest and frozen inputs. It must not send gateway
commands or change the original event.

**Confirm.** Record the event ID and its current state before replay. On the
local demo, replace the event ID below with the one under investigation:

```sh
curl -fsS --max-time 10 -H 'Content-Type: application/json' -H 'X-GridOS-Role: operator' --data '{"eventId":"event_dir_1790476830"}' http://127.0.0.1:28080/gridos.v1.DispatchService/GetEvent
```

Observed output on the standing demo began with `"eventId":"event_dir_1790476830"`,
`"state":"DISPATCH_EVENT_STATE_VALIDATED"`, and `"planVersion":"1"`.

**Act.** Use the same database, decision service, and replay directory as the
running demo. The manifest file must already exist in `GRIDOS_REPLAY_DIR`:

```sh
GRIDOS_REPLAY_DIR=.local/replay GRIDOS_DECISION_ADDR=http://localhost:25061 go run ./services/control/cmd/replay --event event_dir_1790476830
```

Observed output: `IDENTICAL` (exit 0).

**Recover.** `IDENTICAL` means the recomputed plan matches the recorded
version. A differing result exits nonzero and prints the differences; retain
both outputs and the original event's audit chain. Check that the event state
and command count have not changed before closing the investigation. A replay
with different consent, policy, or frozen inputs is a different scenario and
must be labelled as such.

The isolated API test also proves the manifest, timeline, and difference view:
`go test ./services/control/internal/api/replay -run '^TestReplayEventReturnsManifestTimelineAndDiff$' -count=1`
returned `ok github.com/Hirom0112/Base-GridOS/services/control/internal/api/replay 0.301s`.

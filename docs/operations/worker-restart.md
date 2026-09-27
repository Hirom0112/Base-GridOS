# Worker restart

Use this when events stop advancing because the Temporal worker has exited or
stopped polling its task queue. Keep control, gateway, PostgreSQL, and
Temporal running. Record the event ID, last command state, and correlation ID.

**Confirm.** Check the local demo worker PID and its last startup line:

```sh
ps -p "$(pgrep -f '^.local/demo/worker$')" -o pid=,command=
rg 'Started Worker' .local/demo/worker.log | tail -1
```

Observed output before the rehearsal: PID `18758` running
`.local/demo/worker`; the log said `Started Worker Namespace default TaskQueue
gridos-dispatch`. If the PID is gone, inspect the log for the exit reason.

**Act.** After the director releases the standing-demo restart window, stop
only that PID and start the same built worker with the same task queue,
database, gateway, decision, fleet, and metrics settings. Keep PostgreSQL and
Temporal state intact:

```sh
worker_pid=$(pgrep -f '^.local/demo/worker$')
kill "$worker_pid"
for attempt in $(seq 1 50); do kill -0 "$worker_pid" 2>/dev/null || break; sleep 0.2; done
if kill -0 "$worker_pid" 2>/dev/null; then echo 'worker did not stop' >&2; exit 1; fi
python3 - <<'PY'
import os
import subprocess

env = os.environ.copy()
env.update({
    'GRIDOS_WORKER_METRICS_ADDRESS': '0.0.0.0:9465',
    'GRIDOS_DATABASE_URL': 'postgres://gridos:gridos@127.0.0.1:5432/gridos?sslmode=disable',
    'GRIDOS_GATEWAY_TOKEN': 'Bearer local-gateway',
    'GRIDOS_GATEWAY_ADDR': 'http://127.0.0.1:28081',
    'GRIDOS_DECISION_ADDR': 'http://127.0.0.1:25061',
    'GRIDOS_FLEET': 'testdata/fleets/austin-5000.jsonl',
    'GRIDOS_TASK_QUEUE': 'gridos-dispatch',
})
with open('.local/demo/worker.log', 'ab') as log:
    worker = subprocess.Popen(['.local/demo/worker'], env=env,
                              stdin=subprocess.DEVNULL, stdout=log,
                              stderr=subprocess.STDOUT, start_new_session=True)
with open('.local/demo/pids', 'a') as pids:
    pids.write(f'{worker.pid}\n')
print(worker.pid)
PY
```

Observed output: PID `27065` started independently after PID `18758` stopped.
`lsof -nP -iTCP:9465 -sTCP:LISTEN` showed worker PID `27065`, and the
newest log line was `23:29:20 INFO Started Worker ... WorkerID 27065`.
The first shell `nohup` attempt exited before startup without a log entry;
the detached Python launch above kept the process running.

**Recover.** Check the new PID and the newest `Started Worker` log line.
Verify the event resumes from its durable workflow history and no second
command intent appears for the same event and generation. If it remains
stalled, use [Stuck outbox](stuck-outbox.md) and
[Uncertain command](uncertain-command.md). Never mark an unacknowledged
command delivered or relax reserve to force completion.

The isolated termination proof remains:
`go test ./tests/integration -run '^TestWorkerTermination$' -count=1`
returned `ok github.com/Hirom0112/Base-GridOS/tests/integration 111.437s`.

# Gateway restart

Use this when the gateway process exits or telemetry and command receipts
stop. Its SQLite file holds durable commands and buffered telemetry. Never
delete or replace it as a restart step.

**Confirm.** Inspect the current gateway command, process metric, and latest
physical observation. A process metric alone does not prove telemetry is
flowing:

```sh
ps -p "$(pgrep -f '^.local/demo/gateway')" -o pid=,command=
curl -fsS --max-time 5 http://127.0.0.1:9466/metrics | rg '^gridos_gateway_up'
PGOPTIONS='-c statement_timeout=10s' psql 'postgres://gridos:gridos@127.0.0.1:5432/gridos?sslmode=disable' -Atqc "SELECT EXTRACT(EPOCH FROM now()-MAX(observed_at))::int FROM telemetry_observations"
```

Observed output before the rehearsal: PID `18755`,
`gridos_gateway_up 1`, and telemetry lag `1` second.

**Act.** After the director releases the standing-demo restart window, stop
only the gateway and restart the same build with the same SQLite path,
gateway ID, control address, token, and live scenario file:

```sh
gateway_pid=$(pgrep -f '^.local/demo/gateway')
kill "$gateway_pid"
for attempt in $(seq 1 50); do kill -0 "$gateway_pid" 2>/dev/null || break; sleep 0.2; done
if kill -0 "$gateway_pid" 2>/dev/null; then echo 'gateway did not stop' >&2; exit 1; fi
python3 - <<'PY'
import os
import subprocess

env = os.environ.copy()
env.update({'GRIDOS_GATEWAY_METRICS_ADDRESS': '0.0.0.0:9466',
            'GRIDOS_GATEWAY_TOKEN': 'Bearer local-gateway'})
command = ['.local/demo/gateway', '--address', ':28081',
           '--control-address', 'http://127.0.0.1:28080',
           '--database', '.local/demo/gateway.db', '--scenario',
           'testdata/scenarios/heat-event-canonical.yaml', '--live',
           '--gateway-id', 'demo-gateway', '--cadence', '15s']
with open('.local/demo/gateway.log', 'ab') as log:
    gateway = subprocess.Popen(command, env=env, stdin=subprocess.DEVNULL,
                               stdout=log, stderr=subprocess.STDOUT,
                               start_new_session=True)
with open('.local/demo/pids', 'a') as pids:
    pids.write(f'{gateway.pid}\n')
print(gateway.pid)
PY
```

Observed output: PID `28091` running with the same scenario file and `--live`;
`gridos_gateway_up 1` and telemetry lag `11` seconds after one cadence.

**Recover.** Re-run the confirmation block after a cadence. The new gateway
PID must remain alive, `gridos_gateway_up` must equal `1`, and observation lag
must return to the cadence. Inspect `gateway.log` for buffered replay and
producer errors. Duplicate command delivery must return its durable receipt;
an expired command stays expired, and an older generation cannot replace a
newer one. If receipt remains unknown, use
[Uncertain command](uncertain-command.md).

The isolated restart proof remains:
`go test ./tests/integration -run '^TestGatewayRestart$' -count=1`
returned `ok github.com/Hirom0112/Base-GridOS/tests/integration 110.617s`.

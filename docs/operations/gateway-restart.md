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

Observed output before the rehearsal: PID `55275`,
`gridos_gateway_up 1`, and telemetry lag `15` seconds.

**Act.** After the director releases the standing-demo restart window, stop
only the gateway and restart the same build with the same SQLite path,
gateway ID, control address, and token. If the current command uses
`--scenario --live`, set `GRIDOS_DEMO_SCENARIO` to that same file first; leave
it unset for the plain fleet mode:

```sh
gateway_pid=$(pgrep -f '^.local/demo/gateway')
kill "$gateway_pid"
for attempt in $(seq 1 50); do kill -0 "$gateway_pid" 2>/dev/null || break; sleep 0.2; done
if kill -0 "$gateway_pid" 2>/dev/null; then echo 'gateway did not stop' >&2; exit 1; fi
set -- --fleet testdata/fleets/austin-5000.jsonl --scenario-start "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
if test -n "${GRIDOS_DEMO_SCENARIO:-}"; then set -- --scenario "$GRIDOS_DEMO_SCENARIO" --live; fi
nohup env GRIDOS_GATEWAY_METRICS_ADDRESS=0.0.0.0:9466 GRIDOS_GATEWAY_TOKEN='Bearer local-gateway' .local/demo/gateway --address :28081 --control-address http://127.0.0.1:28080 --database .local/demo/gateway.db "$@" --gateway-id demo-gateway --cadence 15s >> .local/demo/gateway.log 2>&1 < /dev/null &
echo $! >> .local/demo/pids
```

Observed output: pending director restart window.

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

# Load test results

Both runs used isolated PostgreSQL databases, random control and gateway ports, and unique Temporal task queues. Neither run used the standing demo.

## Telemetry ingest

The Austin fleet reported 5,000 observations every five seconds for ten minutes. All 600,000 observations were persisted, with zero dropped observations and zero sequence gaps.

```text
$ go test ./tests/load/ -run TelemetryIngest -timeout 20m -count=1 -v
=== RUN   TestTelemetryIngest
    telemetry_ingest_test.go:16: devices=5000 cadence=5s duration=10m persisted=600000 dropped=0 sequence_gaps=0
--- PASS: TestTelemetryIngest (605.54s)
PASS
ok  	github.com/Hirom0112/Base-GridOS/tests/load	605.870s
```

## Dispatch path and operator reads

At 95% observed state of energy, all 5,000 Austin devices were safely above their reserves and received schedules. The test found 5,000 persisted command intents before the first SENT transition; 27 accepted gateway receipts confirmed the network path was live. Across 100 FleetService reads during dispatch, p95 latency was 3.65375 ms, below the 500 ms target. The run stopped after proving the first network delivery and did not measure completion of all 5,000 deliveries.

```text
$ go test ./tests/load/ -run DispatchPath -timeout 20m -count=1 -v
=== RUN   TestDispatchPath
    dispatch_path_test.go:16: persisted=5000 sent=28 acknowledged=27 sent_before_persisted=0 read_samples=100 read_p95=3.65375ms
--- PASS: TestDispatchPath (26.22s)
PASS
ok  	github.com/Hirom0112/Base-GridOS/tests/load	26.578s
```

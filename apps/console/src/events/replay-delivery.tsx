import { useQuery } from "@tanstack/react-query";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { z } from "zod";
import { useSession } from "../api/auth";
import type { ListEventCommandsResponse } from "../api/gen/gridos/v1/events_pb";
import { intervalSchema } from "./commands";

export function ReplayDelivery({
  eventId,
  asOf,
}: {
  eventId: string;
  asOf: Timestamp;
}) {
  const { client, identity } = useSession();
  const query = useQuery({
    queryKey: ["event-commands", eventId, identity.role],
    queryFn: ({ signal }) =>
      client.events.listEventCommands({ eventId }, { signal }),
    enabled: ["operator", "approver", "analyst", "service"].includes(
      identity.role,
    ),
  });
  return (
    <section aria-label="Replay interval measurements">
      <h3>Retrospective interval measurements</h3>
      {query.isPending && (
        <p role="status">Loading recorded interval measurements…</p>
      )}
      {query.isError && (
        <p role="alert">
          Replay measurements unavailable: {query.error.message}
        </p>
      )}
      {query.data && <ReplayDeliveryEvidence data={query.data} asOf={asOf} />}
    </section>
  );
}

export function ReplayDeliveryEvidence({
  data,
  asOf,
}: {
  data: ListEventCommandsResponse;
  asOf: Timestamp;
}) {
  const result = z
    .array(intervalSchema)
    .max(10000)
    .safeParse(data.verificationIntervals);
  if (!result.success)
    return (
      <p role="alert">
        Invalid replay measurement evidence. No delivery is displayed.
      </p>
    );
  const cutoff = asOf.seconds * 1_000_000_000n + BigInt(asOf.nanos);
  const rows = result.data
    .filter(
      (row) =>
        row.endTime.seconds * 1_000_000_000n + BigInt(row.endTime.nanos) <=
        cutoff,
    )
    .sort(
      (a, b) =>
        Number(a.endTime.seconds - b.endTime.seconds) ||
        a.endTime.nanos - b.endTime.nanos,
    )
    .slice(-200);
  if (!rows.length)
    return (
      <p>
        No measured intervals end by this replay time. Delivery is unavailable.
      </p>
    );
  return (
    <>
      <p>
        Last {rows.length} completed intervals at the replay clock. Publication
        times are not supplied; these are retrospective measurements.
      </p>
      <ReplayPowerChart rows={rows} />
      <div
        className="report-scroll"
        role="region"
        aria-label="Replay measurement table"
        tabIndex={0}
      >
        <table
          className="replay-power-table"
          aria-label="Replay interval power"
        >
          <thead>
            <tr>
              <th>Interval end (UTC)</th>
              <th>Requested</th>
              <th>Commanded</th>
              <th>Measured</th>
              <th>Tracking error</th>
              <th>Method</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row, index) => (
              <tr key={index}>
                <td>
                  {new Date(
                    Number(row.endTime.seconds) * 1000 +
                      row.endTime.nanos / 1e6,
                  ).toISOString()}
                </td>
                <td>{row.requestedKw} kW</td>
                <td>{row.commandedKw} kW</td>
                <td>{row.deliveredKw} kW · MEASURED</td>
                <td>{row.trackingErrorKw} kW</td>
                <td>
                  {row.measurementBoundary} · {row.baselineMethod} ·{" "}
                  {row.confidence * 100}% confidence
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

function ReplayPowerChart({
  rows,
}: {
  rows: z.infer<typeof intervalSchema>[];
}) {
  const values = rows.flatMap((row) => [
    row.requestedKw,
    row.commandedKw,
    row.deliveredKw,
  ]);
  const low = Math.min(0, ...values);
  const high = Math.max(1, ...values);
  const starts = rows.map(
    (row) =>
      row.beginTime.seconds * 1_000_000_000n + BigInt(row.beginTime.nanos),
  );
  const finishes = rows.map(
    (row) => row.endTime.seconds * 1_000_000_000n + BigInt(row.endTime.nanos),
  );
  const begins = starts.reduce((minimum, value) =>
    value < minimum ? value : minimum,
  );
  const ends = finishes.reduce((maximum, value) =>
    value > maximum ? value : maximum,
  );
  const x = (time: { seconds: bigint; nanos: number }) =>
    55 +
    (Number(time.seconds * 1_000_000_000n + BigInt(time.nanos) - begins) /
      Number(ends - begins)) *
      690;
  const y = (value: number) => 20 + ((high - value) / (high - low)) * 160;
  const series = [
    {
      key: "requestedKw",
      color: "var(--paper-2)",
      label: "Requested",
      dash: "4 4",
    },
    {
      key: "commandedKw",
      color: "var(--forecast)",
      label: "Commanded",
      dash: "10 4",
    },
    {
      key: "deliveredKw",
      color: "var(--energy)",
      label: "Measured",
      dash: "none",
    },
  ] as const;
  return (
    <svg
      viewBox="0 0 800 240"
      role="img"
      aria-label="Replay requested commanded and measured power"
      style={{ width: "100%", color: "var(--paper-0)" }}
    >
      <line
        x1={55}
        x2={745}
        y1={y(0)}
        y2={y(0)}
        stroke="currentColor"
        opacity={0.4}
      />
      <text x={0} y={20} fill="currentColor" fontSize={12}>
        {high.toFixed(2)} kW
      </text>
      <text x={0} y={180} fill="currentColor" fontSize={12}>
        {low.toFixed(2)} kW
      </text>
      {series.map((item, index) => (
        <g key={item.key}>
          {rows.map((row, i) => (
            <line
              key={i}
              x1={x(row.beginTime)}
              x2={x(row.endTime)}
              y1={y(row[item.key])}
              y2={y(row[item.key])}
              stroke={item.color}
              strokeWidth={3}
              strokeDasharray={item.dash}
            />
          ))}
          <line
            x1={55 + index * 225}
            x2={80 + index * 225}
            y1={230}
            y2={230}
            stroke={item.color}
            strokeWidth={3}
            strokeDasharray={item.dash}
          />
          <text x={90 + index * 225} y={234} fill="currentColor" fontSize={13}>
            {item.label}
          </text>
        </g>
      ))}
      <text x={55} y={202} fill="currentColor" fontSize={12}>
        {new Date(Number(begins / 1_000_000n)).toISOString().slice(11, 19)} UTC
      </text>
      <text x={745} y={202} textAnchor="end" fill="currentColor" fontSize={12}>
        {new Date(Number(ends / 1_000_000n)).toISOString().slice(11, 19)} UTC
      </text>
    </svg>
  );
}

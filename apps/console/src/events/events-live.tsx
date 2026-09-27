import { Code, ConnectError } from "@connectrpc/connect";
import { experimental_streamedQuery, useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { isValidCell } from "h3-js";
import { useSession } from "../api/auth";
import { Evidence, evidenceSchema } from "../api/Provenance";
import type {
  AggregateMetadata,
  H3EventPowerAggregate,
  WatchEventResponse,
} from "../api/gen/gridos/v1/api_pb";
import { ResponseChart } from "./response-chart";

const powerSchema = z
  .object({
    sentMw: z.number().finite(),
    acknowledgedMw: z.number().finite(),
    deliveredMw: z.number().finite(),
    deliveredState: z.number().int().min(0).max(5),
    metadata: evidenceSchema,
    uncertaintyIntervals: z.array(
      z
        .object({
          intervalBeginTime: evidenceSchema.shape.timestamp,
          intervalEndTime: evidenceSchema.shape.timestamp,
          signedFeasiblePowerLowerKw: z.number().finite(),
          signedFeasiblePowerUpperKw: z.number().finite(),
        })
        .refine(
          (interval) =>
            interval.signedFeasiblePowerLowerKw <=
            interval.signedFeasiblePowerUpperKw,
        )
        .refine(
          ({ intervalBeginTime: begin, intervalEndTime: end }) =>
            begin.seconds < end.seconds ||
            (begin.seconds === end.seconds && begin.nanos < end.nanos),
        ),
    ),
  })
  .refine((power) => power.deliveredState !== 5 || power.deliveredMw === 0);

const updateSchema = z.object({
  event: z.object({ eventId: z.string().min(1) }),
  observedAt: evidenceSchema.shape.timestamp,
  fleet: powerSchema,
  h3: z
    .array(
      z.object({
        h3Cell: z.string().refine(isValidCell),
        power: powerSchema,
        metadata: evidenceSchema,
      }),
    )
    .refine(
      (cells) =>
        new Set(cells.map((cell) => cell.h3Cell)).size === cells.length,
    ),
});

export type EventSample = {
  h3: H3EventPowerAggregate[];
  time: number;
  order: bigint;
  sent: number;
  acknowledged: number;
  delivered: number | null;
  metadata: AggregateMetadata;
  intervals: z.infer<typeof updateSchema>["fleet"]["uncertaintyIntervals"];
};

export function reduceEventSamples(
  samples: EventSample[],
  update: WatchEventResponse,
  eventId: string,
): EventSample[] {
  const parsed = updateSchema.parse(update);
  if (parsed.event.eventId !== eventId || !update.fleet?.metadata)
    throw new Error("Stream event identity or evidence is invalid");
  const order =
    parsed.observedAt.seconds * 1000000000n + BigInt(parsed.observedAt.nanos);
  const previous = samples.at(-1);
  if (previous && order < previous.order)
    throw new Error("Stream observation order is invalid");
  const power = parsed.fleet;
  if (power.deliveredState === 5 && power.deliveredMw !== 0)
    throw new Error("Zero delivery state has a nonzero measurement");
  const next = {
    h3: update.h3,
    time:
      Number(parsed.observedAt.seconds) * 1000 +
      parsed.observedAt.nanos / 1000000,
    order,
    sent: power.sentMw,
    acknowledged: power.acknowledgedMw,
    delivered: [1, 5].includes(power.deliveredState) ? power.deliveredMw : null,
    metadata: update.fleet.metadata,
    intervals: power.uncertaintyIntervals,
  };
  return [
    ...(previous?.order === order ? samples.slice(0, -1) : samples).slice(-119),
    next,
  ];
}

export function LiveEvent({ eventId }: { eventId: string }) {
  const { client, identity } = useSession();
  const allowed = ["operator", "approver", "analyst", "service"].includes(
    identity.role,
  );
  const query = useQuery({
    queryKey: ["event-stream", eventId, identity.role],
    enabled: allowed,
    queryFn: experimental_streamedQuery({
      streamFn: async function* ({ signal }) {
        while (!signal.aborted) {
          try {
            yield* client.events.watchEvent(
              { eventId },
              { signal, timeoutMs: 60000 },
            );
            throw new ConnectError("Event stream ended", Code.Unavailable);
          } catch (error) {
            if (
              error instanceof ConnectError &&
              error.code === Code.DeadlineExceeded &&
              !signal.aborted
            )
              continue;
            throw error;
          }
        }
      },
      refetchMode: "append",
      initialValue: [] as EventSample[],
      reducer: (samples, update) =>
        reduceEventSamples(samples, update, eventId),
    }),
  });
  if (!allowed)
    return (
      <p className="boundary-note">
        Live event evidence is unavailable to this role.
      </p>
    );
  return (
    <section className="live-response" aria-label="Measured event response">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Intent → receipt → response</p>
          <h3>Measured event response</h3>
        </div>
        <span className="mode-chip">
          {query.isError
            ? "Disconnected"
            : query.failureCount > 0
              ? "Reconnecting"
              : query.isPending
                ? "Connecting"
                : "Stream connected"}
        </span>
      </div>
      {query.isPending && (
        <p role="status">Waiting for the first server observation…</p>
      )}
      {(query.isError || query.failureCount > 0) && (
        <div className="boundary-note" role="alert">
          Live connection unavailable. Retained observations are not current.{" "}
          <button className="secondary-button" onClick={() => query.refetch()}>
            Reconnect event stream
          </button>
        </div>
      )}
      {query.data && <EventResponse samples={query.data} />}
    </section>
  );
}

export function EventResponse({ samples }: { samples: EventSample[] }) {
  const latest = samples.at(-1);
  if (!latest) return <p>No measured event observations returned.</p>;
  return (
    <>
      <div className="response-values">
        <div className="sent-value">
          <span>Sent intent</span>
          <strong>{latest.sent.toFixed(3)} MW</strong>
        </div>
        <div className="ack-value">
          <span>Acknowledged receipt</span>
          <strong>{latest.acknowledged.toFixed(3)} MW</strong>
        </div>
        <div className="delivery-value">
          <span>Measured delivery</span>
          <strong>
            {latest.delivered === null
              ? "Delivery unknown"
              : `${latest.delivered.toFixed(3)} MW`}
          </strong>
        </div>
      </div>
      <Evidence metadata={latest.metadata} />
      <ResponseChart samples={samples} />
      <p className="history-gap">
        Last {samples.length} observations from this session. Gaps mean unknown
        delivery, not zero. Acknowledgement proves receipt only.
      </p>
      <details className="response-data">
        <summary>Exact response observations</summary>
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Observation UTC</th>
                <th>Sent MW</th>
                <th>Acknowledged MW</th>
                <th>Delivered MW</th>
              </tr>
            </thead>
            <tbody>
              {samples.map((sample) => (
                <tr key={sample.order.toString()}>
                  <td>{new Date(sample.time).toISOString()}</td>
                  <td>{sample.sent.toFixed(3)}</td>
                  <td>{sample.acknowledged.toFixed(3)}</td>
                  <td>{sample.delivered?.toFixed(3) ?? "Unknown"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </details>
      {latest.intervals.length > 0 && (
        <details className="response-data">
          <summary>
            {latest.intervals.length} uncertain device intervals
          </summary>
          <p>Device bounds are not a fleet confidence interval.</p>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Begin UTC</th>
                  <th>End UTC</th>
                  <th>Lower kW</th>
                  <th>Upper kW</th>
                </tr>
              </thead>
              <tbody>
                {latest.intervals.map((interval, index) => (
                  <tr key={index}>
                    <td>
                      {new Date(
                        Number(interval.intervalBeginTime.seconds) * 1000 +
                          interval.intervalBeginTime.nanos / 1000000,
                      ).toISOString()}
                    </td>
                    <td>
                      {new Date(
                        Number(interval.intervalEndTime.seconds) * 1000 +
                          interval.intervalEndTime.nanos / 1000000,
                      ).toISOString()}
                    </td>
                    <td>{interval.signedFeasiblePowerLowerKw}</td>
                    <td>{interval.signedFeasiblePowerUpperKw}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </details>
      )}
    </>
  );
}

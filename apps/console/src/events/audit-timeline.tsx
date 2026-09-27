import { useId } from "react";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { useSession } from "../api/auth";
import { evidenceSchema } from "../api/Provenance";
import type { EventTimelineEntry } from "../api/gen/gridos/v1/api_pb";
import type { DispatchEvent } from "../api/gen/gridos/v1/dispatch_pb";
import { EventExceptions } from "./exceptions";
import { EventHistory, eventStateLabels } from "./events-timeline";

export const recordsSchema = z
  .array(
    z.object({
      sequence: z.bigint().positive(),
      occurredAt: evidenceSchema.shape.timestamp,
      actorId: z.string().min(1),
      action: z.string().min(1),
      previousState: z.number().int(),
      state: z.number().int(),
      reason: z.string(),
    }),
  )
  .refine((records) =>
    records.every(
      (record, index) =>
        index === 0 || record.sequence > records[index - 1]!.sequence,
    ),
  );

export function AuditTimeline({ event }: { event: DispatchEvent }) {
  const { client, identity } = useSession();
  const query = useQuery({
    queryKey: ["event-audit", event.eventId, identity.role],
    queryFn: ({ signal }) =>
      client.events.getEventTimeline({ eventId: event.eventId }, { signal }),
    refetchInterval: 3000,
  });
  if (query.isPending)
    return <p role="status">Loading server audit records…</p>;
  if (query.isError)
    return (
      <>
        <div className="boundary-note" role="status">
          Audit feed unavailable. Showing only records present in the event
          snapshot.
          <button className="secondary-button" onClick={() => query.refetch()}>
            Retry audit feed
          </button>
        </div>
        <EventHistory event={event} />
      </>
    );
  return (
    <>
      <EventExceptions
        eventId={event.eventId}
        records={query.data.exceptions}
      />
      <AuditRecords entries={query.data.entries} />
    </>
  );
}

export function AuditRecords({ entries }: { entries: EventTimelineEntry[] }) {
  const titleId = useId();
  const parsed = recordsSchema.safeParse(entries);
  if (!parsed.success)
    return (
      <p role="alert">
        Audit evidence is invalid. Refresh before relying on this history.
      </p>
    );
  return (
    <section className="event-history" aria-labelledby={titleId}>
      <div className="section-heading">
        <div>
          <p className="eyebrow">Evidence trail</p>
          <h3 id={titleId}>Server audit timeline</h3>
        </div>
        <span className="mode-chip">{parsed.data.length} records</span>
      </div>
      {parsed.data.length ? (
        <ol aria-label="Server audit timeline">
          {parsed.data.map((record) => {
            const at = new Date(
              Number(record.occurredAt.seconds) * 1000 +
                record.occurredAt.nanos / 1000000,
            ).toISOString();
            return (
              <li key={record.sequence.toString()}>
                <div>
                  <strong>{record.action.replaceAll("_", " ")}</strong>
                  <time className="mono" dateTime={at}>
                    {at}
                  </time>
                </div>
                {(record.previousState !== 0 || record.state !== 0) && (
                  <p>
                    {eventStateLabels[record.previousState] ?? "Unspecified"} →{" "}
                    {eventStateLabels[record.state] ?? "Unspecified"}
                  </p>
                )}
                <p>{record.reason || "No reason supplied by the server."}</p>
                <p className="mono">
                  Record {record.sequence.toString()} · {record.actorId}
                </p>
              </li>
            );
          })}
        </ol>
      ) : (
        <p className="history-gap">No audit records returned by the server.</p>
      )}
      <p className="history-gap">
        Server-recorded actions and decisions. Command acceptance does not prove
        measured delivery.
      </p>
    </section>
  );
}

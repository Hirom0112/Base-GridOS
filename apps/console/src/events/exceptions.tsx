import { z } from "zod";
import { evidenceSchema } from "../api/Provenance";
import {
  EventExceptionKind,
  type EventException,
} from "../api/gen/gridos/v1/events_pb";

const exceptionsSchema = z.array(
  z.object({
    kind: z.number().int().min(1).max(10),
    occurredAt: evidenceSchema.shape.timestamp,
    eventId: z.string().min(1),
    deviceId: z.string(),
    commandId: z.string(),
    evidenceId: z.string(),
    detail: z.string(),
  }),
);

export function EventExceptions({
  eventId,
  records,
}: {
  eventId: string;
  records: EventException[];
}) {
  const parsed = exceptionsSchema.safeParse(records);
  if (
    !parsed.success ||
    parsed.data.some((record) => record.eventId !== eventId)
  )
    return (
      <p role="alert">
        Exception evidence is invalid. Refresh before relying on these records.
      </p>
    );
  return (
    <section className="event-history" aria-labelledby="exceptions-title">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Failures and recovery</p>
          <h3 id="exceptions-title">Recorded exceptions</h3>
        </div>
        <span className="mode-chip">{parsed.data.length} records</span>
      </div>
      <p className="history-gap">
        A retry or replacement decision does not prove recovery. Confirm
        subsequent acknowledgement and measured delivery.
      </p>
      {parsed.data.length ? (
        <ol aria-label="Exception evidence">
          {parsed.data.map((record, index) => {
            const at = new Date(
              Number(record.occurredAt.seconds) * 1000 +
                record.occurredAt.nanos / 1000000,
            ).toISOString();
            return (
              <li key={`${record.evidenceId}:${index}`}>
                <div>
                  <strong>{EventExceptionKind[record.kind]}</strong>
                  <time className="mono" dateTime={at}>
                    {at}
                  </time>
                </div>
                <p>{record.detail || "No detail supplied by the server."}</p>
                <p className="mono">
                  Device: {record.deviceId || "Not supplied"} · Command:{" "}
                  {record.commandId || "Not supplied"}
                </p>
                <p className="mono">
                  Evidence: {record.evidenceId || "Not supplied"}
                </p>
              </li>
            );
          })}
        </ol>
      ) : (
        <p className="history-gap">
          No exception records returned by the server.
        </p>
      )}
    </section>
  );
}

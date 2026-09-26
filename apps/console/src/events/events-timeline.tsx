import type { DispatchEvent } from "../api/gen/gridos/v1/dispatch_pb";
import { evidenceSchema } from "../api/Provenance";

export const eventStateLabels: Record<number, string> = {
  1: "Requested",
  2: "Planned",
  3: "Safety validated",
  4: "Approved",
  5: "Commands persisted",
  6: "Sent",
  7: "Acknowledged or uncertain",
  8: "Executing",
  9: "Telemetry verified",
  10: "Reconciled",
  11: "Reported",
};

export function EventHistory({ event }: { event: DispatchEvent }) {
  const records = [
    {
      label: "Event created",
      timestamp: event.createdAt,
      detail: "Request entered the event lifecycle.",
    },
    ...(event.launch
      ? [
          {
            label: "Launch requested",
            timestamp: event.launch.requestedAt,
            detail: `${event.launch.requestedBy} · plan v${event.launch.planVersion}`,
          },
        ]
      : []),
    {
      label: `Latest recorded state · ${eventStateLabels[event.state] ?? "Unknown"}`,
      timestamp: event.updatedAt,
      detail: `Current server snapshot · plan v${event.planVersion}`,
    },
  ];
  const known = records
    .flatMap((record) => {
      const parsed = evidenceSchema.shape.timestamp.safeParse(record.timestamp);
      if (!parsed.success) return [];
      const { seconds, nanos } = parsed.data;
      return [
        {
          ...record,
          order: seconds * 1000000000n + BigInt(nanos),
          at: new Date(Number(seconds) * 1000 + nanos / 1000000).toISOString(),
        },
      ];
    })
    .sort((a, b) => (a.order < b.order ? -1 : a.order > b.order ? 1 : 0));
  return (
    <section className="event-history" aria-labelledby="history-title">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Evidence trail</p>
          <h3 id="history-title">Known event records</h3>
        </div>
        <span className="mode-chip">Partial history</span>
      </div>
      {known.length ? (
        <ol aria-label="Known event records">
          {known.map((record) => (
            <li key={record.label}>
              <div>
                <strong>{record.label}</strong>
                <time className="mono" dateTime={record.at}>
                  {record.at.replace("T", " · ").replace(".000Z", " UTC")}
                </time>
              </div>
              <p>{record.detail}</p>
            </li>
          ))}
        </ol>
      ) : (
        <p className="history-gap">No valid record timestamps returned.</p>
      )}
      {known.length > 0 && known.length < records.length && (
        <p className="history-gap">Some record timestamps are unavailable.</p>
      )}
      <p className="history-gap">
        This is not a complete transition history. Approval, retries, and
        recovery records require the server audit feed; missing steps are not
        inferred.
      </p>
    </section>
  );
}

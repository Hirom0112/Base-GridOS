import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { useSession } from "../api/auth";

const count = z.number().int().nonnegative().safe();
const reserveSchema = z
  .object({
    DevicesExpected: count.positive(),
    DevicesObserved: count.positive(),
    MinimumMarginKWh: z.number().finite(),
    DevicesTouchedFloor: count,
    ObservationGaps: count,
    ValueKind: z.literal("MEASURED"),
    Provenance: z.array(z.string().min(1)).min(1),
  })
  .refine(
    (value) =>
      value.DevicesObserved <= value.DevicesExpected &&
      value.DevicesTouchedFloor <= value.DevicesObserved &&
      value.ObservationGaps === value.DevicesExpected - value.DevicesObserved,
  );

export function ReserveEvidence({ evidence }: { evidence: unknown }) {
  const result = reserveSchema.safeParse(evidence);
  return (
    <section className="event-report" aria-label="Reserve protection evidence">
      <h3>Measured reserve margins</h3>
      {evidence == null ? (
        <p>
          Measured reserve evidence unavailable. Planning reserve is not proof
          of observed compliance.
        </p>
      ) : !result.success ? (
        <p role="alert">Reserve evidence is invalid.</p>
      ) : (
        <>
          <p>
            {result.data.DevicesObserved} of {result.data.DevicesExpected}{" "}
            devices observed · MEASURED
          </p>
          <dl className="report-values">
            <div>
              <dt>Minimum observed margin above effective reserve</dt>
              <dd>{result.data.MinimumMarginKWh.toFixed(3)} kWh</dd>
            </div>
          </dl>
          <p>
            {result.data.DevicesTouchedFloor} devices touched or crossed the
            reserve floor.
          </p>
          <p>
            {result.data.ObservationGaps} devices without observations.
            Unobserved devices remain unknown.
          </p>
          <p>{result.data.Provenance.join(" · ")}</p>
          <p>
            Margins compare recorded state of energy against the frozen
            effective reserve. They describe available observations, not
            continuous coverage.
          </p>
        </>
      )}
    </section>
  );
}

export function EventReserve({ eventId }: { eventId: string }) {
  const { client, identity } = useSession();
  const allowed = ["operator", "approver", "analyst", "service"].includes(
    identity.role,
  );
  const query = useQuery({
    queryKey: ["published-report", eventId, identity.role],
    queryFn: ({ signal }) =>
      client.reports.getEventReport(
        { eventId, partnerView: false },
        { signal },
      ),
    enabled: allowed,
    refetchInterval: 5000,
  });
  if (!allowed) return null;
  if (query.isPending)
    return <p role="status">Loading measured reserve evidence…</p>;
  if (query.isError)
    return (
      <p role="alert">
        Measured reserve evidence unavailable.{" "}
        <button onClick={() => query.refetch()}>Retry reserve evidence</button>
      </p>
    );
  try {
    const value: unknown = JSON.parse(query.data.reportJson);
    const report = z
      .object({
        EventID: z.literal(eventId),
        ReserveCompliance: z.unknown().optional(),
      })
      .parse(value);
    return <ReserveEvidence evidence={report.ReserveCompliance} />;
  } catch {
    return (
      <p role="alert">
        Reserve report does not match this event or is invalid.
      </p>
    );
  }
}

import { useState, type FormEvent } from "react";
import { useHydrated } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { useSession } from "../api/auth";
import type { CompareEventReportsResponse } from "../api/gen/gridos/v1/api_pb";
import "./report.css";

const versionSchema = z.union([
  z.literal(""),
  z
    .string()
    .regex(/^[1-9]\d*$/)
    .max(20)
    .pipe(z.string().refine((value) => BigInt(value) <= 18446744073709551615n)),
]);
export const comparisonSchema = z.object({
  eventIdA: z.string().trim().min(1).max(200),
  eventIdB: z.string().trim().min(1).max(200),
  planVersionA: versionSchema,
  planVersionB: versionSchema,
});
const differencesSchema = z.array(
  z.object({ field: z.string().min(1), before: z.string(), after: z.string() }),
);

export function ComparisonEvidence({
  data,
}: {
  data: CompareEventReportsResponse;
}) {
  const parsed = differencesSchema.safeParse(data.differences);
  if (!parsed.success)
    return <p role="alert">Comparison evidence is invalid.</p>;
  if (!parsed.data.length)
    return <p>No differences returned for the selected reports.</p>;
  return (
    <div
      className="report-scroll"
      role="region"
      aria-label="Report differences"
      tabIndex={0}
    >
      <table aria-label="Report differences">
        <thead>
          <tr>
            <th>Field</th>
            <th>Before · A</th>
            <th>After · B</th>
          </tr>
        </thead>
        <tbody>
          {parsed.data.map((row, index) => (
            <tr key={index}>
              <th>{row.field}</th>
              <td>{row.before}</td>
              <td>{row.after}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function ReportComparison({ eventId = "" }: { eventId?: string }) {
  const { client, identity } = useSession();
  const hydrated = useHydrated();
  const [selection, setSelection] = useState<z.infer<
    typeof comparisonSchema
  > | null>(null);
  const [error, setError] = useState("");
  const allowed = ["operator", "approver", "analyst", "service"].includes(
    identity.role,
  );
  const query = useQuery({
    queryKey: ["report-comparison", selection, identity.role],
    queryFn: ({ signal }) => {
      if (!selection) throw new Error("Select two reports");
      return client.reports.compareEventReports(
        {
          eventIdA: selection.eventIdA,
          eventIdB: selection.eventIdB,
          planVersionA: selection.planVersionA
            ? BigInt(selection.planVersionA)
            : undefined,
          planVersionB: selection.planVersionB
            ? BigInt(selection.planVersionB)
            : undefined,
        },
        { signal },
      );
    },
    enabled: allowed && selection !== null,
  });
  function compare(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const result = comparisonSchema.safeParse(
      Object.fromEntries(new FormData(event.currentTarget)),
    );
    if (!result.success) {
      setError(
        "Enter both event IDs and positive whole plan versions, or leave versions blank for latest published.",
      );
      return;
    }
    setError("");
    setSelection(result.data);
  }
  if (!allowed) return null;
  return (
    <section className="event-report" aria-label="Compare event reports">
      <h3>Compare event reports</h3>
      <p>
        Leave a plan version blank to compare the latest published report. Field
        names and values are returned by the server.
      </p>
      <form className="report-comparison" onSubmit={compare}>
        <label>
          Event A<input name="eventIdA" required defaultValue={eventId} />
        </label>
        <label>
          Plan version A<input name="planVersionA" inputMode="numeric" />
        </label>
        <label>
          Event B<input name="eventIdB" required />
        </label>
        <label>
          Plan version B<input name="planVersionB" inputMode="numeric" />
        </label>
        <button type="submit" disabled={!hydrated || query.isFetching}>
          Compare reports
        </button>
      </form>
      {error && <p role="alert">{error}</p>}
      {query.isFetching && <p role="status">Comparing published reports…</p>}
      {selection && (
        <p>
          A: {selection.eventIdA} · v{selection.planVersionA || "latest"} → B:{" "}
          {selection.eventIdB} · v{selection.planVersionB || "latest"}
        </p>
      )}
      {query.isError && (
        <p role="alert">
          Comparison unavailable: {query.error.message}{" "}
          <button onClick={() => query.refetch()}>Retry comparison</button>
        </p>
      )}
      {query.data && <ComparisonEvidence data={query.data} />}
    </section>
  );
}

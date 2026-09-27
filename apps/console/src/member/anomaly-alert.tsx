import { useQuery } from "@tanstack/react-query";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { z } from "zod";
import { useSession } from "../api/auth";
import { evidenceSchema } from "../api/Provenance";
import type { HomeActivityAlert } from "../api/gen/gridos/v1/pricing_pb";

const alertsSchema = z.array(
  z.object({
    alertId: z.string().min(1),
    memberId: z.string().min(1),
    description: z.literal("energy anomaly signal"),
    observedAt: evidenceSchema.shape.timestamp,
    consentVersion: z.string().min(1),
  }),
);

export function AnomalyEvidence({
  memberId,
  alerts,
}: {
  memberId: string;
  alerts: HomeActivityAlert[];
}) {
  const parsed = alertsSchema.safeParse(alerts);
  if (
    !parsed.success ||
    parsed.data.some((alert) => alert.memberId !== memberId)
  )
    return <p role="alert">Home activity evidence is invalid.</p>;
  return (
    <>
      <p>
        Energy anomaly signals identify unusual electricity use. This is not a
        security monitoring service.
      </p>
      {alerts.length ? (
        <ul>
          {alerts.map((alert) => (
            <li key={alert.alertId}>
              <strong>{alert.description}</strong>
              <p>
                {timestampDate(alert.observedAt!).toISOString()} · Consent{" "}
                {alert.consentVersion}
              </p>
            </li>
          ))}
        </ul>
      ) : (
        <p>
          No home activity alerts returned. This does not establish whether your
          home is occupied.
        </p>
      )}
    </>
  );
}

export function AnomalyAlerts({ memberId }: { memberId: string }) {
  const { client, identity } = useSession();
  const query = useQuery({
    queryKey: ["member-alerts", memberId, identity.role],
    queryFn: ({ signal }) =>
      client.member.listHomeActivityAlerts({ memberId }, { signal }),
    refetchInterval: 30000,
  });
  return (
    <section aria-label="Home activity alerts">
      <h3>Home activity alerts</h3>
      {query.isPending && <p role="status">Loading home activity evidence…</p>}
      {query.isError && (
        <p role="alert">
          Home activity alerts unavailable.{" "}
          <button onClick={() => query.refetch()}>Retry alerts</button>
        </p>
      )}
      {query.data && (
        <AnomalyEvidence memberId={memberId} alerts={query.data.alerts} />
      )}
    </section>
  );
}

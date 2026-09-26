import { useQuery } from "@tanstack/react-query";
import { useSession } from "../api/auth";
import { Evidence, evidenceSchema, Quantity } from "../api/Provenance";
import type {
  FleetDeviceCountAggregate,
  FleetSummary,
} from "../api/gen/gridos/v1/api_pb";

export function useFleet() {
  const { client, identity } = useSession();
  const enabled = identity.role !== "member";
  const summary = useQuery({
    queryKey: ["fleet", identity.role],
    queryFn: ({ signal }) =>
      client.fleet.getFleetSummary({ loadZones: ["LZ_AEN"] }, { signal }),
    enabled,
  });
  const sites = useQuery({
    queryKey: ["sites", identity.role],
    queryFn: async ({ signal }) => {
      const response = await client.fleet.listSites(
        { loadZones: ["LZ_AEN"], pageSize: 1000 },
        { signal },
      );
      if (response.nextPageToken)
        throw new Error("Incomplete fleet response: additional pages required");
      if (response.sites.some((site) => site.location.case !== "aggregate"))
        throw new Error("Aggregate geography required");
      return response;
    },
    enabled,
  });
  return { summary, sites };
}

const operatingLabels: Record<number, string> = {
  1: "On grid",
  2: "Off-grid outage",
  3: "No home power",
  4: "Overcurrent",
  5: "Overcurrent standby",
  6: "Telemetry unavailable",
};
const availabilityLabels: Record<number, string> = {
  1: "Online",
  2: "Offline",
  3: "Degraded",
  4: "Stale",
  5: "Maintenance",
};
const healthLabels: Record<number, string> = {
  1: "Healthy",
  2: "Degraded",
  3: "Unhealthy",
  4: "Unknown",
};

export function FleetMetrics({
  summary,
}: {
  summary: FleetSummary | undefined;
}) {
  if (!summary) return <p role="status">Waiting for fleet evidence…</p>;
  return (
    <>
      <section className="fleet-quantities" aria-label="Fleet capacity">
        <Quantity
          label="Installed power"
          unit="MW"
          aggregate={summary.installedMw}
        />
        <Quantity
          label="Usable energy"
          unit="MWh"
          aggregate={summary.installedMwh}
        />
        <Quantity
          label="Reserved for backup"
          unit="MWh"
          aggregate={summary.reservedForBackupMwh}
        />
        <Quantity
          label="Dispatchable now"
          unit="MW"
          aggregate={summary.dispatchableNowMw}
        />
        <Quantity
          label="Forecast dispatchable"
          unit="MW"
          aggregate={summary.forecastDispatchableMw}
        />
      </section>
      <div className="fleet-health">
        <section>
          <h2>Operating state</h2>
          {summary.operatingStateCounts.map((item) => (
            <Count
              key={item.operatingState}
              label={
                operatingLabels[item.operatingState] ??
                "Unknown operating state"
              }
              aggregate={item.aggregate}
            />
          ))}
        </section>
        <section>
          <h2>Availability</h2>
          {summary.availabilityStateCounts.map((item) => (
            <Count
              key={item.availabilityState}
              label={
                availabilityLabels[item.availabilityState] ??
                "Unknown availability"
              }
              aggregate={item.aggregate}
            />
          ))}
        </section>
        <section>
          <h2>Communications</h2>
          {summary.communicationsHealthCounts.map((item) => (
            <Count
              key={item.healthState}
              label={healthLabels[item.healthState] ?? "Unknown health"}
              aggregate={item.aggregate}
            />
          ))}
        </section>
        <section>
          <h2>Acknowledgement health</h2>
          {summary.acknowledgementHealthCounts.map((item) => (
            <Count
              key={item.healthState}
              label={healthLabels[item.healthState] ?? "Unknown health"}
              aggregate={item.aggregate}
            />
          ))}
        </section>
      </div>
    </>
  );
}

function Count({
  label,
  aggregate,
}: {
  label: string;
  aggregate: FleetDeviceCountAggregate | undefined;
}) {
  return (
    <details className="fleet-count">
      <summary>
        <span>{label}</span>
        <strong className="mono">
          {evidenceSchema.safeParse(aggregate?.metadata).success
            ? aggregate?.deviceCount.toLocaleString()
            : "—"}
        </strong>
      </summary>
      <Evidence metadata={aggregate?.metadata} />
    </details>
  );
}

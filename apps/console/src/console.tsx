import {
  lazy,
  Suspense,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  ClientOnly,
  Link,
  useLocation,
  useParams,
} from "@tanstack/react-router";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { ReplayClock, useReplayClock } from "./events/replay-clock";
import { MemberHome } from "./member/home";
import { Shell } from "./shell";
import { useSession } from "./api/auth";
import { Evidence, evidenceSchema, Quantity } from "./api/Provenance";
import { useFleet } from "./fleet/fleet";
import { useObservability } from "./observability";
import type {
  AggregateMetadata,
  FleetSummary,
  H3SiteAggregate,
} from "./api/gen/gridos/v1/api_pb";

const GeographicField = lazy(() =>
  import("./fleet/geographic-field").catch(() => ({
    default: UnavailableGeographicField,
  })),
);

export function Console({ children }: { children: ReactNode }) {
  const { identity } = useSession();
  const { position, setPosition } = useReplayClock();
  const { summary, sites } = useFleet();
  const { pathname } = useLocation();
  useObservability(pathname, identity.role);
  const params = useParams({ strict: false });
  const [activeEvent, setActiveEvent] = useState(params.eventId);
  useEffect(() => {
    if (params.eventId) setActiveEvent(params.eventId);
    if (params.eventId && position && params.eventId !== position.eventId)
      setPosition(null);
  }, [params.eventId, position, setPosition]);
  const eventId = params.eventId ?? activeEvent;
  const [selectedCell, setSelectedCell] = useState<string | null>(null);
  const fleet = summary.data?.summary;
  const metadata = fleet?.installedMw?.metadata;
  const observedAt =
    evidenceSchema.safeParse(metadata).success && metadata?.timestamp
      ? timestampDate(metadata.timestamp).toISOString()
      : undefined;
  const cells = useMemo(
    () =>
      sites.data?.sites.flatMap((site) =>
        site.location.case === "aggregate" ? [site.location.value] : [],
      ) ?? [],
    [sites.data],
  );
  const selected = cells.find((cell) => cell.h3Cell === selectedCell);
  if (identity.role === "member") return <MemberHome />;
  return (
    <Shell
      observedAt={
        position ? timestampDate(position.at).toISOString() : observedAt
      }
      identity={identity.role}
      eventId={eventId}
      evidence={
        position ? (
          <aside className="evidence-rail">
            <h2>Historical geography</h2>
            <p>
              The field uses the replay clock. Inspect its timestamped cell
              evidence below. Per-cell command and delivery history remain
              unavailable.
            </p>
          </aside>
        ) : (
          <FleetEvidence
            fleet={fleet}
            metadata={metadata}
            selected={selected}
            clear={() => setSelectedCell(null)}
          />
        )
      }
    >
      <div
        className={`situational-field connected-field ${pathname === "/fleet" ? "fleet-view" : "event-view"}`}
        data-view={pathname}
      >
        <ConsoleHeading pathname={pathname} role={identity.role} />
        <ReplayClock />
        <FleetFailure summary={summary} sites={sites} />
        {!position && pathname === "/fleet" && <FleetHeadlines fleet={fleet} />}
        <ClientOnly
          fallback={
            <div className="grid-loading" role="status">
              Preparing the geographic field…
            </div>
          }
        >
          <Suspense
            fallback={
              <div className="grid-loading" role="status">
                Preparing the geographic field…
              </div>
            }
          >
            <GeographicField
              eventId={eventId}
              stage={stageOf(pathname)}
              active={pathname !== "/map"}
              cells={cells}
              selected={selectedCell}
              onSelect={setSelectedCell}
            />
          </Suspense>
        </ClientOnly>
        <div className="route-evidence">{children}</div>
      </div>
    </Shell>
  );
}

function FleetFailure({
  summary,
  sites,
}: Pick<ReturnType<typeof useFleet>, "summary" | "sites">) {
  if (!summary.isError && !sites.isError) return null;
  return (
    <div role="alert" className="error-notice">
      {summary.error?.message ?? sites.error?.message}
      <button
        onClick={() => {
          void summary.refetch();
          void sites.refetch();
        }}
      >
        Retry fleet
      </button>
    </div>
  );
}

function stageOf(pathname: string) {
  if (pathname === "/dispatch/new") return "Optimize";
  if (pathname.startsWith("/dispatch/")) return "Approve";
  if (pathname.endsWith("/report")) return "Learn";
  if (pathname.startsWith("/events/") && pathname !== "/events/compare")
    return "Dispatch";
  return "Observe";
}

function ConsoleHeading({
  pathname,
  role,
}: {
  pathname: string;
  role: string;
}) {
  const title =
    pathname === "/events/compare"
      ? "Compare event evidence"
      : pathname === "/member"
        ? "Household access"
        : pathname === "/map"
          ? "Explore the fleet"
          : pathname === "/fleet"
            ? "Austin fleet"
            : pathname === "/dispatch/new"
              ? "Shape a safe dispatch"
              : pathname.endsWith("/report")
                ? "Event evidence"
                : pathname.startsWith("/events")
                  ? "From command to response"
                  : "Review the plan";
  return (
    <div className="view-heading">
      <div>
        <h1 id="fleet-title">{title}</h1>
      </div>
      {role === "operator" && pathname !== "/dispatch/new" && (
        <Link className="action-button" to="/dispatch/new">
          Plan a dispatch <span aria-hidden="true">↗</span>
        </Link>
      )}
    </div>
  );
}

function FleetEvidence({
  fleet,
  metadata,
  selected,
  clear,
}: {
  fleet: FleetSummary | undefined;
  metadata: AggregateMetadata | undefined;
  selected: H3SiteAggregate | undefined;
  clear: () => void;
}) {
  const batteries = fleet?.operatingStateCounts.reduce(
    (total, item) => total + (item.aggregate?.deviceCount ?? 0n),
    0n,
  );
  const online = fleet?.availabilityStateCounts.find(
    (item) => item.availabilityState === 1,
  )?.aggregate?.deviceCount;
  const rows = [
    ["fleet", "Batteries", batteries?.toLocaleString()],
    ["online", "Online", online?.toLocaleString()],
    ["installed", "Installed", megawatts(fleet?.installedMw?.value)],
    ["available", "Available now", megawatts(fleet?.dispatchableNowMw?.value)],
    [
      "reserve",
      "Backup held",
      fleet?.reservedForBackupMwh
        ? `${fleet.reservedForBackupMwh.value.toFixed(1)} MWh`
        : undefined,
    ],
  ] as const;
  return (
    <aside className="evidence-rail" aria-labelledby="evidence-title">
      <div className="evidence-heading">
        <h2 id="evidence-title">Fleet evidence</h2>
        <span className="mono">LZ_AEN</span>
      </div>
      <dl className="evidence-facts">
        {rows.map(([icon, label, value]) => (
          <div key={label}>
            <EvidenceIcon kind={icon} />
            <dt>{label}</dt>
            <dd className="mono">{value ?? "—"}</dd>
          </div>
        ))}
      </dl>
      {selected && (
        <section className="evidence-card">
          <EvidenceIcon kind="focus" />
          <p className="eyebrow">Selected cell</p>
          <h3 className="mono">{selected.h3Cell}</h3>
          <p>{selected.siteCount.toLocaleString()} batteries</p>
          <Quantity
            label="Installed power"
            unit="MW"
            aggregate={selected.installedMw}
          />
          <button className="text-button" onClick={clear}>
            Clear selection
          </button>
        </section>
      )}
      <section className="evidence-source">
        <p className="eyebrow">Source</p>
        <Evidence metadata={metadata} />
      </section>
    </aside>
  );
}

function megawatts(value: number | undefined) {
  return value === undefined ? undefined : `${value.toFixed(1)} MW`;
}

const evidenceIcons = {
  fleet: "M7 6h10v14H7Z M10 3h4v3 M10 11h4 M10 15h4",
  online: "M3 12h4l3-7 4 14 3-7h4",
  installed: "M13 3 5 14h6l-1 7 8-11h-6Z",
  available: "M4 17l5-5 4 4 7-8 M15 8h5v5",
  focus: "M12 3v4M12 17v4M3 12h4M17 12h4M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Z",
  reserve: "M12 3 5 6v6c0 4 3 7.5 7 9 4-1.5 7-5 7-9V6Z M9 12l2 2 4-4",
};

function EvidenceIcon({ kind }: { kind: keyof typeof evidenceIcons }) {
  return (
    <svg className="evidence-icon" viewBox="0 0 24 24" aria-hidden="true">
      <path d={evidenceIcons[kind]} />
    </svg>
  );
}

function FleetHeadlines({ fleet }: { fleet: FleetSummary | undefined }) {
  return (
    <div className="headline-quantities">
      <Quantity
        label="Installed power"
        unit="MW"
        aggregate={fleet?.installedMw}
      />
      <Quantity
        label="Usable energy"
        unit="MWh"
        aggregate={fleet?.installedMwh}
      />
      <Quantity
        label="Reserved for backup"
        unit="MWh"
        aggregate={fleet?.reservedForBackupMwh}
      />
    </div>
  );
}

function UnavailableGeographicField({
  cells,
  active = true,
}: {
  cells: H3SiteAggregate[];
  active?: boolean;
}) {
  const { position } = useReplayClock();
  return (
    <section
      className="living-grid"
      aria-label="Unavailable geographic field"
      hidden={!active}
    >
      <div className="field-header">
        <h2>Geographic assets unavailable</h2>
      </div>
      <p className="boundary-note">
        Reload the page to retry the geographic view. Plan controls and server
        evidence remain available.
      </p>
      {position ? (
        <p className="boundary-note">
          Historical cell evidence is unavailable while geographic assets cannot
          load. Current cell capacity is withheld during replay.
        </p>
      ) : (
        <details className="geography-table">
          <summary>Inspect recorded cell capacity</summary>
          <div
            className="table-scroll"
            role="region"
            aria-label="Recorded cell capacity"
            tabIndex={0}
          >
            <table>
              <thead>
                <tr>
                  <th>Recorded cell</th>
                  <th>Installed power</th>
                </tr>
              </thead>
              <tbody>
                {cells.map((cell, index) => (
                  <tr key={`${cell.h3Cell}:${index}`}>
                    <th scope="row">{cell.h3Cell}</th>
                    <td>
                      <Quantity
                        label="Installed power"
                        unit="MW"
                        aggregate={cell.installedMw}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </details>
      )}
    </section>
  );
}

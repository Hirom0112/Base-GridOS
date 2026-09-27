import {
  lazy,
  Suspense,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
  type ComponentProps,
} from "react";
import { Link, useLocation, useParams } from "@tanstack/react-router";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useEventStream } from "./events/events-live";
import { MemberHome } from "./member/home";
import { Shell } from "./shell";
import { useSession } from "./api/auth";
import { Evidence, evidenceSchema, Quantity } from "./api/Provenance";
import { useFleet } from "./fleet/fleet";
import type {
  AggregateMetadata,
  H3SiteAggregate,
} from "./api/gen/gridos/v1/api_pb";

const LivingGrid = lazy(() => import("./fleet/living-grid"));

export function Console({ children }: { children: ReactNode }) {
  const { identity } = useSession();
  const { summary, sites } = useFleet();
  const { pathname } = useLocation();
  const params = useParams({ strict: false });
  const [activeEvent, setActiveEvent] = useState(params.eventId);
  useEffect(() => {
    if (params.eventId) setActiveEvent(params.eventId);
  }, [params.eventId]);
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
      observedAt={observedAt}
      identity={identity.role}
      eventId={eventId}
      evidence={
        <FleetEvidence
          pathname={pathname}
          metadata={metadata}
          selected={selected}
          clear={() => setSelectedCell(null)}
        />
      }
    >
      <div
        className={`situational-field connected-field ${pathname === "/fleet" ? "fleet-view" : "event-view"}`}
        data-view={pathname}
      >
        <ConsoleHeading pathname={pathname} role={identity.role} />
        {(summary.isError || sites.isError) && (
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
        )}
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
        <Suspense
          fallback={
            <div className="grid-loading" role="status">
              Preparing the geographic field…
            </div>
          }
        >
          <GeographicField
            eventId={eventId}
            active={pathname !== "/map"}
            cells={cells}
            selected={selectedCell}
            onSelect={setSelectedCell}
          />
        </Suspense>
        <div className="route-evidence">{children}</div>
      </div>
    </Shell>
  );
}

function ConsoleHeading({
  pathname,
  role,
}: {
  pathname: string;
  role: string;
}) {
  const title =
    pathname === "/map"
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
        <p className="eyebrow">
          Living Grid /{" "}
          {["/fleet", "/map"].includes(pathname) ? "Observe" : "Event thread"}
        </p>
        <h1 id="fleet-title">{title}</h1>
        <p>Greater Austin · A governed fleet, one operating loop.</p>
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
  pathname,
  metadata,
  selected,
  clear,
}: {
  pathname: string;
  metadata: AggregateMetadata | undefined;
  selected: H3SiteAggregate | undefined;
  clear: () => void;
}) {
  return (
    <aside className="evidence-rail" aria-labelledby="evidence-title">
      <div className="evidence-heading">
        <h2 id="evidence-title">Fleet evidence</h2>
        <span className="mono">LZ_AEN</span>
      </div>
      <section className="evidence-card">
        <p className="eyebrow">Observation</p>
        <h3>Source before certainty.</h3>
        <Evidence metadata={metadata} />
      </section>
      <section className="evidence-card">
        <p className="eyebrow">Operator focus</p>
        <h3>
          {pathname === "/map"
            ? "Regional inspection"
            : selected
              ? selected.h3Cell
              : "Greater Austin"}
        </h3>
        {pathname === "/map" ? (
          <p>
            Select a cell on the map or in its evidence table. Its timestamped
            details appear directly below the map.
          </p>
        ) : selected ? (
          <>
            <p>{selected.siteCount.toLocaleString()} sites in this H3 cell</p>
            <Quantity
              label="Installed power"
              unit="MW"
              aggregate={selected.installedMw}
            />
            <button className="text-button" onClick={clear}>
              Clear selection
            </button>
          </>
        ) : (
          <p>
            Select a region in the grid or its table to inspect the recorded
            capacity.
          </p>
        )}
      </section>
      <section className="evidence-card reserve-card">
        <p className="eyebrow">Protected by design</p>
        <h3>Reserve comes first.</h3>
        <p>
          The server validates household backup limits before an operator can
          approve a plan.
        </p>
      </section>
      <section className="evidence-card">
        <p className="eyebrow">Reading the field</p>
        {pathname === "/map" ? (
          <p>
            Cell boundaries: H3 geography
            <br />
            Lighter fill: the selected measure
            <br />
            White outline: operator selection
          </p>
        ) : (
          <p>
            Position: H3 location
            <br />
            Footprint: site count
            <br />
            Height: installed MW
          </p>
        )}
        <p>
          Cell-level availability and delivery remain unencoded until supplied
          by the server.
        </p>
      </section>
    </aside>
  );
}

function GeographicField({
  eventId,
  ...props
}: ComponentProps<typeof LivingGrid> & { eventId?: string }) {
  const { query } = useEventStream(eventId);
  return (
    <LivingGrid
      {...props}
      response={query.isError ? undefined : query.data?.at(-1)}
    />
  );
}

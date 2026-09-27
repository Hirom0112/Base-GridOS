import { useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { timestampFromDate, timestampDate } from "@bufbuild/protobuf/wkt";
import { useSession } from "../api/auth";
import { provenanceNames } from "../api/Provenance";
import {
  DispatchEventState,
  ExclusionReason,
  type DispatchEvent,
} from "../api/gen/gridos/v1/dispatch_pb";
import type { BasicEventReport } from "../api/gen/gridos/v1/api_pb";
import { ApprovalActions } from "../dispatch/approval";
import { PlanExplanation } from "../dispatch/explanation";
import { eventStateLabels } from "./events-timeline";
import { AuditTimeline } from "./audit-timeline";
import { EmergencyStopControl } from "./emergency-stop";
import { LiveEvent } from "./events-live";

const pendingStates: Partial<Record<DispatchEventState, string>> = {
  [DispatchEventState.REQUESTED]:
    "Planning is queued. Approval remains unavailable until the server validates a plan.",
  [DispatchEventState.PLANNED]:
    "A plan is recorded. Awaiting the server safety result before approval.",
  [DispatchEventState.COMMANDS_PERSISTED]:
    "Commands are persisted. Awaiting confirmation that they have been sent.",
};

function useEvent(eventId: string) {
  const { client, identity } = useSession();
  const keys = useRef(
    new Map<
      string,
      { key: string; at: ReturnType<typeof timestampFromDate> }
    >(),
  );
  const query = useQuery({
    queryKey: ["event", eventId, identity.role],
    queryFn: ({ signal }) => client.dispatch.getEvent({ eventId }, { signal }),
    refetchInterval: 3000,
  });
  async function confirm(action: "approve" | "launch") {
    const event = query.data?.event;
    if (!event) throw new Error("Event evidence unavailable");
    const key = `${event.eventId}:${event.planVersion}:${action}`;
    let intent = keys.current.get(key);
    if (!intent) {
      intent = { key: crypto.randomUUID(), at: timestampFromDate(new Date()) };
      keys.current.set(key, intent);
    }
    const { key: idempotencyKey, at: timestamp } = intent;
    const actor =
      identity.mode === "local" ? `local-${identity.role}` : identity.userId;
    if (action === "approve")
      await client.dispatch.approveEvent({
        eventId,
        planVersion: event.planVersion,
        idempotencyKey,
        approvedBy: actor,
        approvedAt: timestamp,
      });
    else
      await client.dispatch.launchEvent({
        eventId,
        planVersion: event.planVersion,
        idempotencyKey,
        requestedBy: actor,
        requestedAt: timestamp,
      });
    await query.refetch();
  }
  return { query, identity, confirm };
}

export function EventView({
  eventId,
  view,
}: {
  eventId: string;
  view: "plan" | "execution" | "report";
}) {
  const { query, identity, confirm } = useEvent(eventId);
  if (query.isPending) return <p role="status">Loading the versioned event…</p>;
  if (query.isError)
    return (
      <div role="alert" className="error-notice">
        {query.error.message}
        <button onClick={() => query.refetch()}>Retry event</button>
      </div>
    );
  const { event, exclusions, report, safetyViolations } = query.data;
  if (!event) return <p role="alert">The server returned no event record.</p>;
  const provenance = event.provenance?.provenance;
  const source =
    provenanceNames[provenance as keyof typeof provenanceNames] ??
    "Provenance unavailable";
  return (
    <section className="event-panel">
      <div className="section-heading">
        <div>
          <p className="eyebrow">
            {view === "report" ? "Event accounting" : "Versioned event"}
          </p>
          <h2>{eventStateLabels[event.state] ?? "Unknown state"}</h2>
        </div>
        <span className="mode-chip">{source}</span>
      </div>
      <EventIdentifiers event={event} />
      <div className="event-links">
        <Link to="/dispatch/$eventId" params={{ eventId }}>
          Plan & approval
        </Link>
        <Link
          to="/events/$eventId"
          params={{ eventId }}
          activeOptions={{ exact: true }}
        >
          Execution
        </Link>
        <Link to="/events/$eventId/report" params={{ eventId }}>
          Report
        </Link>
      </div>
      <PendingEvent state={event.state} violations={safetyViolations} />
      {view === "execution" && (
        <>
          <EmergencyStopControl key={eventId} eventId={eventId} />
          <LiveEvent key={`live-${eventId}`} eventId={eventId} />
        </>
      )}
      {safetyViolations.length > 0 && (
        <div className="error-notice">
          <h3>Safety gate rejected the plan</h3>
          <ul>
            {safetyViolations.map((violation) => (
              <li key={violation.code}>{violation.code}</li>
            ))}
          </ul>
        </div>
      )}
      <div className="event-summary">
        <div>
          <span className="eyebrow">Launch record</span>
          <p>
            {event.launch
              ? `Requested by ${event.launch.requestedBy} · plan v${event.launch.planVersion}`
              : "No launch record returned"}
          </p>
          <small>
            {event.launch?.requestedAt
              ? timestampDate(event.launch.requestedAt).toISOString()
              : "Approval and launch are separate actions."}
          </small>
        </div>
        <div>
          <span className="eyebrow">Physical response</span>
          <p>
            {event.state >= DispatchEventState.SENT
              ? "Awaiting measured delivery evidence"
              : "Commands have not been reported sent"}
          </p>
          <small>Acknowledgement proves receipt, not delivered energy.</small>
        </div>
      </div>
      {view === "plan" && (
        <>
          <PlanExplanation eventId={eventId} planVersion={event.planVersion} />
          <ApprovalActions
            event={event}
            role={identity.role}
            onConfirm={confirm}
          />
        </>
      )}
      <section className="exclusions">
        <h3>Exclusions by reason</h3>
        {exclusions.length ? (
          <ul>
            {exclusions.map((group) => (
              <li key={group.reason}>
                {ExclusionReason[group.reason]?.replaceAll("_", " ") ??
                  "Unknown reason"}
                <strong>{group.count.toLocaleString()} devices</strong>
              </li>
            ))}
          </ul>
        ) : (
          <p>No exclusions returned by the server.</p>
        )}
      </section>
      {report && <EventAccounting report={report} />}
      <AuditTimeline event={event} />
      {view !== "plan" && (
        <div className="boundary-note">
          Verified delivery, replay, and modeled economics are not supplied by
          this event response. They remain unavailable until the corresponding
          service provides evidence.
        </div>
      )}
    </section>
  );
}

function EventIdentifiers({ event }: { event: DispatchEvent }) {
  return (
    <div className="event-identifiers">
      <span className="mono">{event.eventId}</span>
      <strong>
        {event.planVersion > 0n ? `Plan v${event.planVersion}` : "Plan pending"}
      </strong>
      <span>
        {event.updatedAt
          ? timestampDate(event.updatedAt).toISOString()
          : "Timestamp unavailable"}
      </span>
    </div>
  );
}

function PendingEvent({
  state,
  violations,
}: {
  state: DispatchEventState;
  violations: { code: string }[];
}) {
  const message = pendingStates[state];
  if (!message || violations.length) return null;
  return (
    <p className="pending-event" role="status">
      {message}
    </p>
  );
}

function EventAccounting({ report }: { report: BasicEventReport }) {
  return (
    <>
      <h3>Server event accounting</h3>
      <div className="report-quantities">
        {[
          ["Requested", report.requestedMw],
          ["Approved", report.approvedMw],
          ["Commanded", report.commandedMw],
          ["Acknowledged", report.acknowledgedMw],
        ].map(([label, value]) => (
          <div key={label}>
            <span>{label}</span>
            <strong className="mono">
              {Number(value).toFixed(3)} <small>MW</small>
            </strong>
          </div>
        ))}
      </div>
      <p className="report-provenance">
        {report.provenance.join(" · ")} · Policy {report.policyVersion} · Solver{" "}
        {report.solverVersion} · Model {report.modelVersion}
      </p>
    </>
  );
}

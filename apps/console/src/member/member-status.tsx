import { z } from "zod";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { evidenceSchema } from "../api/Provenance";
import type { GetMemberStatusResponse } from "../api/gen/gridos/v1/member_pb";

const statusSchema = z.object({
  memberId: z.string().min(1),
  siteId: z.string().min(1),
  operatingState: z.number().int().min(1).max(6),
  availability: z.number().int().min(1).max(5),
  stateOfEnergyPercent: z.number().min(0).max(100),
  backupHoursCurrent: z.number().nonnegative(),
  backupHours750w: z.number().nonnegative(),
  observedAt: evidenceSchema.shape.timestamp,
  effectiveReservePercent: z.number().min(0).max(100),
  currentPlan: z
    .object({
      displayName: z.string().min(1),
      reserveFloorPercent: z.number().min(0).max(100),
      policyVersion: z.string().min(1),
      catalogVersion: z.string(),
      termsKnown: z.boolean(),
      energyMonthlyChargeCents: z.bigint().nonnegative(),
      batteryMonthlyChargeCents: z.bigint().nonnegative(),
      flexibilityRewardCents: z.bigint().nonnegative(),
    })
    .optional(),
  recentEvents: z.array(
    z.object({
      eventId: z.string().min(1),
      beginTime: evidenceSchema.shape.timestamp,
      endTime: evidenceSchema.shape.timestamp,
      participated: z.boolean(),
      participationExplanation: z.string(),
    }),
  ),
});
const states: Record<number, string> = {
  1: "On grid",
  2: "Off-grid outage",
  3: "No home power",
  4: "Overcurrent",
  5: "Overcurrent standby",
  6: "Telemetry unavailable",
};

export function MemberStatus({
  data,
  memberId,
  siteId,
}: {
  data: GetMemberStatusResponse;
  memberId: string;
  siteId: string;
}) {
  const parsed = statusSchema.safeParse(data);
  if (!parsed.success || data.memberId !== memberId || data.siteId !== siteId)
    return (
      <p role="alert">
        Household evidence is invalid. Refresh before relying on this status.
      </p>
    );
  const available = data.operatingState !== 6 && data.availability === 1;
  return (
    <section aria-label="Household status">
      <p className="eyebrow">Your home</p>
      <h2>{states[data.operatingState]}</h2>
      <p>
        Last observation:{" "}
        <time dateTime={timestampDate(data.observedAt!).toISOString()}>
          {timestampDate(data.observedAt!).toISOString()}
        </time>
      </p>
      <dl className="report-values">
        <div>
          <dt>State of energy</dt>
          <dd>{available ? `${data.stateOfEnergyPercent}%` : "Unavailable"}</dd>
        </div>
        <div>
          <dt>Backup at recorded usage</dt>
          <dd>
            {available && data.backupHoursCurrent > 0
              ? `${data.backupHoursCurrent} h`
              : "Unavailable"}
          </dd>
        </div>
        <div>
          <dt>Backup at 750 W</dt>
          <dd>
            {available && data.backupHours750w > 0
              ? `${data.backupHours750w} h`
              : "Unavailable"}
          </dd>
        </div>
      </dl>
      <p className="boundary-note">
        Backup durations are estimates at the recorded load. Current power-flow
        measurements are unavailable in this response.
      </p>
      <MemberPlan data={data} />
      <h3>Recent grid events</h3>
      {data.recentEvents.length ? (
        <ul>
          {data.recentEvents.map((event) => (
            <li key={event.eventId}>
              <strong>
                {event.participated ? "Participated" : "Did not participate"}
              </strong>
              <p>
                {event.participationExplanation ||
                  "No participation explanation supplied"}
              </p>
              <p>
                {timestampDate(event.beginTime!).toISOString()} –{" "}
                {timestampDate(event.endTime!).toISOString()}
              </p>
            </li>
          ))}
        </ul>
      ) : (
        <p>No recent grid events returned.</p>
      )}
      <p>
        Savings and settled participation rewards are unavailable in this status
        response.
      </p>
    </section>
  );
}

function MemberPlan({ data }: { data: GetMemberStatusResponse }) {
  const plan = data.currentPlan;
  if (!plan) return <p>Plan and reserve evidence unavailable.</p>;
  const amounts: [string, bigint][] = [
    ["Energy monthly charge", plan.energyMonthlyChargeCents],
    ["Battery monthly charge", plan.batteryMonthlyChargeCents],
    ["Flexibility reward in plan terms", plan.flexibilityRewardCents],
  ];
  return (
    <section aria-label="Plan and reserve">
      <h3>{plan.displayName}</h3>
      <dl className="report-values">
        <div>
          <dt>Plan reserve floor</dt>
          <dd>{plan.reserveFloorPercent}%</dd>
        </div>
        <div>
          <dt>Effective reserve</dt>
          <dd>{data.effectiveReservePercent}%</dd>
        </div>
        {amounts.map(([label, cents]) => (
          <div key={label}>
            <dt>{label}</dt>
            <dd>
              {plan.termsKnown
                ? `${cents / 100n}.${(cents % 100n).toString().padStart(2, "0")} USD`
                : "Terms unavailable"}
            </dd>
          </div>
        ))}
      </dl>
      <p>
        Policy {plan.policyVersion} · Catalog{" "}
        {plan.catalogVersion || "Unavailable"}
      </p>
      {plan.reserveFloorPercent === 0 && (
        <p>
          A 0% plan reserve leaves no guaranteed battery energy held back for
          backup. Actual backup depends on energy remaining when an outage
          begins.
        </p>
      )}
    </section>
  );
}

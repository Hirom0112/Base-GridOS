import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { useSession } from "../api/auth";
import type { Role } from "../api/client";
import "./report.css";
import { ReserveEvidence } from "./reserve";

const number = z.number().finite();
const reportSchema = z.object({
  EventID: z.string().min(1),
  PlanVersion: z.number().int().nonnegative(),
  RequestedMW: number,
  ApprovedMW: number,
  CommandedMW: number,
  AcknowledgedMW: number,
  Provenance: z.array(z.string()),
  Versions: z.record(z.string(), z.string()),
  Delivered: z
    .object({
      DeliveredMW: number,
      DeliveredMWh: number,
      Completeness: number.min(0).max(1),
      TrackingErrorMW: number,
      ResponseLatency: number.nonnegative(),
    })
    .nullable(),
  Energy: z
    .object({
      RequestedMWh: number,
      ApprovedMWh: number,
      CommandedMWh: number,
      AcknowledgedMWh: number,
      DeliveredMWh: number,
    })
    .nullable(),
  ReserveCompliance: z.unknown().optional(),
  ReserveViolationsPrevented: z.number().int().nonnegative(),
  MemberRewardsCents: z.number().int().safe().nullable(),
  Margin: z
    .object({
      ValueUSD: number,
      HurdleUSD: number,
      ValueKind: z.literal("modeled_estimate"),
      margin_bound: z.string().optional(),
      price_provenance: z.string().optional(),
      unavailable_cost_terms: z.array(z.string()).optional(),
    })
    .nullable(),
  Economics: z
    .object({
      GrossValueUSD: number,
      DegradationCostUSD: number,
      PenaltyExposureUSD: number,
      NetValueUSD: number,
      ValueKind: z.literal("modeled_estimate"),
    })
    .nullable(),
  DataGaps: z
    .array(
      z.object({
        Begin: z.iso.datetime({ offset: true }),
        End: z.iso.datetime({ offset: true }),
        Reason: z.string().min(1),
      }),
    )
    .nullable(),
  Assumptions: z.array(z.string()).nullable(),
});
const partnerSchema = z.object({
  event_id: z.string().min(1),
  plan_version: z.number().int().nonnegative(),
  requested_mw: number,
  approved_mw: number,
  commanded_mw: number,
  acknowledged_mw: number,
  delivered_mw: number.optional(),
  delivered_mwh: number.optional(),
  modeled_net_value_usd: number.optional(),
  value_kind: z.literal("modeled_estimate").optional(),
});

function ReportValues({
  entries,
}: {
  entries: [string, number | null | undefined, string][];
}) {
  return (
    <dl className="report-values">
      {entries.map(([label, value, unit]) => (
        <div key={label}>
          <dt>{label}</dt>
          <dd>
            {value == null
              ? "Unavailable"
              : `${value.toFixed(unit === "USD" ? 2 : 3)} ${unit}`}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function DetailedReport({ report }: { report: z.infer<typeof reportSchema> }) {
  const gaps = new Set(report.DataGaps?.map((gap) => gap.Reason));
  const energy = report.Energy;
  return (
    <>
      <p className="mono">
        {report.EventID} · Plan v{report.PlanVersion}
      </p>
      <ReportValues
        entries={[
          ["Requested power", report.RequestedMW, "MW"],
          ["Approved power", report.ApprovedMW, "MW"],
          ["Commanded power", report.CommandedMW, "MW"],
          ["Acknowledged power", report.AcknowledgedMW, "MW"],
          [
            "Requested energy",
            gaps.has("requested_energy_unavailable")
              ? null
              : energy?.RequestedMWh,
            "MWh",
          ],
          [
            "Approved energy",
            gaps.has("approved_energy_unavailable")
              ? null
              : energy?.ApprovedMWh,
            "MWh",
          ],
          [
            "Commanded energy",
            gaps.has("commanded_energy_unavailable")
              ? null
              : energy?.CommandedMWh,
            "MWh",
          ],
          [
            "Acknowledged energy",
            gaps.has("acknowledged_energy_unavailable")
              ? null
              : energy?.AcknowledgedMWh,
            "MWh",
          ],
          [
            "Reserve violations prevented",
            gaps.has("reserve_violations_prevented_unavailable")
              ? null
              : report.ReserveViolationsPrevented,
            "violations",
          ],
          [
            "Member rewards",
            report.MemberRewardsCents == null
              ? null
              : report.MemberRewardsCents / 100,
            "USD",
          ],
        ]}
      />
      <DeliveryMeasurements report={report} />
      <ReserveEvidence evidence={report.ReserveCompliance} />
      <ReportEconomics report={report} />
      <ReportLineage report={report} />
    </>
  );
}

function DeliveryMeasurements({
  report,
}: {
  report: z.infer<typeof reportSchema>;
}) {
  const delivered =
    report.Delivered && report.Delivered.Completeness > 0
      ? report.Delivered
      : null;
  const energyUnavailable = report.DataGaps?.some(
    (gap) => gap.Reason === "delivered_energy_unavailable",
  );
  return (
    <ReportValues
      entries={[
        ["Delivered power", delivered?.DeliveredMW, "MW"],
        [
          "Delivered energy",
          energyUnavailable ? null : delivered?.DeliveredMWh,
          "MWh",
        ],
        [
          "Measurement completeness",
          report.Delivered == null ? null : report.Delivered.Completeness * 100,
          "%",
        ],
        ["Tracking error", delivered?.TrackingErrorMW, "MW"],
        [
          "Response latency",
          report.Delivered == null
            ? null
            : report.Delivered.ResponseLatency / 1e9,
          "s",
        ],
      ]}
    />
  );
}

function ReportEconomics({ report }: { report: z.infer<typeof reportSchema> }) {
  return (
    <>
      <h4>Modeled economics</h4>
      <p>Estimates from the stored report; not settled revenue.</p>
      <ReportValues
        entries={[
          ["Modeled margin", report.Margin?.ValueUSD, "USD"],
          ["Margin hurdle", report.Margin?.HurdleUSD, "USD"],
          ["Modeled gross value", report.Economics?.GrossValueUSD, "USD"],
          [
            "Modeled degradation cost",
            report.Economics?.DegradationCostUSD,
            "USD",
          ],
          [
            "Modeled penalty exposure",
            report.Economics?.PenaltyExposureUSD,
            "USD",
          ],
          ["Modeled net value", report.Economics?.NetValueUSD, "USD"],
        ]}
      />
      {report.Margin && (
        <p>
          {report.Margin.ValueKind} · Bound:{" "}
          {report.Margin.margin_bound || "Not supplied"} · Price source:{" "}
          {report.Margin.price_provenance || "Not supplied"}
          {report.Margin.unavailable_cost_terms?.length
            ? ` · Missing costs: ${report.Margin.unavailable_cost_terms.join(", ")}`
            : ""}
        </p>
      )}
    </>
  );
}

function ReportLineage({ report }: { report: z.infer<typeof reportSchema> }) {
  return (
    <>
      <h4>Data gaps</h4>
      {report.DataGaps?.length ? (
        <ul>
          {report.DataGaps.map((gap, index) => (
            <li key={index}>
              <strong>{gap.Reason}</strong>
              <p>
                {gap.Begin} – {gap.End}
              </p>
            </li>
          ))}
        </ul>
      ) : (
        <p>No data gaps listed in this report.</p>
      )}
      <h4>Provenance and versions</h4>
      <p>{report.Provenance.join(" · ") || "Provenance unavailable"}</p>
      <dl className="report-values">
        {Object.entries(report.Versions).map(([name, value]) => (
          <div key={name}>
            <dt>{name}</dt>
            <dd>{value || "Unavailable"}</dd>
          </div>
        ))}
      </dl>
      <h4>Assumptions</h4>
      {report.Assumptions?.length ? (
        <ul>
          {report.Assumptions.map((value, index) => (
            <li key={index}>{value}</li>
          ))}
        </ul>
      ) : (
        <p>No assumptions supplied.</p>
      )}
    </>
  );
}

export function ReportEvidence({
  eventId,
  role,
  json,
}: {
  eventId: string;
  role: Role;
  json: string;
}) {
  let value: unknown;
  try {
    value = JSON.parse(json);
  } catch {
    return <p role="alert">Report evidence is invalid.</p>;
  }
  if (role === "partner") {
    const result = partnerSchema.safeParse(value);
    if (!result.success || result.data.event_id !== eventId)
      return <p role="alert">Report evidence is invalid.</p>;
    const report = result.data;
    return (
      <>
        <p>
          {report.event_id} · Plan v{report.plan_version} · Partner aggregates
        </p>
        <ReportValues
          entries={[
            ["Requested power", report.requested_mw, "MW"],
            ["Approved power", report.approved_mw, "MW"],
            ["Commanded power", report.commanded_mw, "MW"],
            ["Acknowledged power", report.acknowledged_mw, "MW"],
            ["Delivered power", report.delivered_mw, "MW"],
            ["Delivered energy", report.delivered_mwh, "MWh"],
            [
              "Modeled net value",
              report.value_kind === "modeled_estimate"
                ? report.modeled_net_value_usd
                : null,
              "USD",
            ],
          ]}
        />
        <p>Modeled values are estimates, not settled revenue.</p>
      </>
    );
  }
  const result = reportSchema.safeParse(value);
  if (!result.success || result.data.EventID !== eventId)
    return <p role="alert">Report evidence is invalid.</p>;
  return <DetailedReport report={result.data} />;
}

export function EventReport({ eventId }: { eventId: string }) {
  const { client, identity } = useSession();
  const query = useQuery({
    queryKey: ["published-report", eventId, identity.role],
    queryFn: ({ signal }) =>
      client.reports.getEventReport(
        { eventId, partnerView: identity.role === "partner" },
        { signal },
      ),
  });
  return (
    <section className="event-report" aria-labelledby="report-title">
      <h3 id="report-title">Event report</h3>
      {query.isPending && <p role="status">Loading report evidence…</p>}
      {query.isError && (
        <p role="alert">
          Report unavailable: {query.error.message}{" "}
          <button onClick={() => query.refetch()}>Retry report</button>
        </p>
      )}
      {query.data && (
        <ReportEvidence
          eventId={eventId}
          role={identity.role}
          json={query.data.reportJson}
        />
      )}
    </section>
  );
}

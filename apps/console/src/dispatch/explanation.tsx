import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { useSession } from "../api/auth";
import { evidenceSchema } from "../api/Provenance";
import type { GetPlanExplanationResponse } from "../api/gen/gridos/v1/api_pb";
import "./explanation.css";

const timeSchema = evidenceSchema.shape.timestamp;
const explanationSchema = z.object({
  reserveHeldBackKwh: z.number().nonnegative(),
  objectiveBreakdown: z
    .object({
      gridValue: z.number(),
      commitmentTrackingValue: z.number(),
      chargingEnergyCost: z.number(),
      degradationCost: z.number(),
      penaltyExposure: z.number(),
      reliabilityRiskCost: z.number(),
      objectiveValue: z.number(),
    })
    .optional(),
  constraintMargins: z.array(
    z.object({
      constraintName: z.string().min(1),
      margin: z.number(),
      units: z.string().min(1),
      intervalBeginTime: timeSchema.optional(),
    }),
  ),
  shortfalls: z.array(
    z
      .object({
        intervalBeginTime: timeSchema,
        intervalEndTime: timeSchema,
        requestedKw: z.number().nonnegative(),
        feasibleKw: z.number().nonnegative(),
        shortfallKw: z.number().nonnegative(),
        reasons: z.array(z.string()),
      })
      .refine(
        (value) =>
          value.intervalEndTime.seconds > value.intervalBeginTime.seconds ||
          (value.intervalEndTime.seconds === value.intervalBeginTime.seconds &&
            value.intervalEndTime.nanos > value.intervalBeginTime.nanos),
      ),
  ),
  marginExplanation: z
    .object({
      conservativeMargin: z.number(),
      marginHurdle: z.number(),
      terms: z.array(
        z
          .object({
            name: z.string().min(1),
            low: z.number(),
            high: z.number(),
            source: z.string(),
            unavailable: z.boolean(),
          })
          .refine((value) => value.unavailable || value.low <= value.high),
      ),
    })
    .optional(),
});

const objectiveLabels = {
  gridValue: "Grid value",
  commitmentTrackingValue: "Commitment tracking value",
  chargingEnergyCost: "Charging energy cost",
  degradationCost: "Degradation cost",
  penaltyExposure: "Penalty exposure",
  reliabilityRiskCost: "Reliability risk cost",
  objectiveValue: "Net objective",
} as const;
const quantity = new Intl.NumberFormat("en-US", {
  minimumFractionDigits: 3,
  maximumFractionDigits: 3,
});

export function PlanExplanation({
  eventId,
  planVersion,
}: {
  eventId: string;
  planVersion: bigint;
}) {
  const { client, identity } = useSession();
  const query = useQuery({
    queryKey: ["explanation", eventId, planVersion.toString(), identity.role],
    queryFn: ({ signal }) =>
      client.dispatch.getPlanExplanation({ eventId, planVersion }, { signal }),
    enabled: planVersion > 0n,
  });
  if (planVersion === 0n) return null;
  if (query.isPending)
    return (
      <p role="status">Loading plan v{planVersion.toString()} explanation…</p>
    );
  if (query.isError)
    return (
      <div role="alert" className="error-notice">
        Plan explanation unavailable. {query.error.message}
        <button onClick={() => void query.refetch()}>Retry explanation</button>
      </div>
    );
  return <ExplanationEvidence explanation={query.data} />;
}

export function ExplanationEvidence({
  explanation,
}: {
  explanation: GetPlanExplanationResponse;
}) {
  const result = explanationSchema.safeParse(explanation);
  if (!result.success)
    return (
      <p role="alert">
        Plan explanation is invalid. Values cannot be used for review.
      </p>
    );
  const {
    reserveHeldBackKwh,
    objectiveBreakdown,
    constraintMargins,
    shortfalls,
    marginExplanation,
  } = result.data;
  return (
    <section className="plan-explanation" aria-label="Optimization explanation">
      <header className="explanation-heading">
        <div>
          <p className="eyebrow">Plan evidence</p>
          <h3>Why this plan is feasible</h3>
        </div>
        <div>
          <span>Reserve held back</span>
          <strong className="mono">
            {quantity.format(reserveHeldBackKwh)} kWh
          </strong>
        </div>
      </header>
      <p className="explanation-note">
        Values belong to the selected event and plan version. Explanation issue
        time and provenance are not supplied by this response.
      </p>
      <section aria-label="Constraint margins">
        <h3>Constraint margins</h3>
        <p>
          Margins are shown as returned. They do not replace the independent
          safety result.
        </p>
        {constraintMargins.length ? (
          <div className="explanation-scroll">
            <table>
              <thead>
                <tr>
                  <th>Constraint</th>
                  <th>Margin</th>
                  <th>Interval begins (UTC)</th>
                </tr>
              </thead>
              <tbody>
                {constraintMargins.map((margin, index) => (
                  <tr key={index}>
                    <th scope="row">{margin.constraintName}</th>
                    <td className="mono">
                      {String(margin.margin)} {margin.units}
                    </td>
                    <td>
                      {margin.intervalBeginTime ? (
                        <EvidenceTime value={margin.intervalBeginTime} />
                      ) : (
                        "Not supplied"
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p>No constraint margins returned.</p>
        )}
      </section>
      <section>
        <h3>Interval feasibility</h3>
        {shortfalls.length ? (
          <div className="explanation-scroll">
            <table aria-label="Interval feasibility">
              <thead>
                <tr>
                  <th>Window (UTC)</th>
                  <th>Requested kW</th>
                  <th>Feasible kW</th>
                  <th>Shortfall kW</th>
                  <th>Reasons</th>
                </tr>
              </thead>
              <tbody>
                {shortfalls.map((interval, index) => (
                  <tr key={index}>
                    <th scope="row">
                      <EvidenceTime value={interval.intervalBeginTime} />
                      <EvidenceTime value={interval.intervalEndTime} />
                    </th>
                    <td className="mono">
                      {quantity.format(interval.requestedKw)}
                    </td>
                    <td className="mono">
                      {quantity.format(interval.feasibleKw)}
                    </td>
                    <td className="mono">
                      {quantity.format(interval.shortfallKw)}
                    </td>
                    <td>{interval.reasons.join(" · ") || "None returned"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p>No interval feasibility returned.</p>
        )}
      </section>
      <details className="objective-evidence" open={!objectiveBreakdown}>
        <summary>Modeled objective · inspect the value and costs</summary>
        <p>
          Modeled estimates, not settled revenue. Objective units are not
          supplied by the contract.
        </p>
        {objectiveBreakdown ? (
          <dl>
            {Object.entries(objectiveLabels).map(([key, label]) => (
              <div key={key}>
                <dt>{label}</dt>
                <dd className="mono">
                  {quantity.format(
                    objectiveBreakdown[key as keyof typeof objectiveLabels],
                  )}
                </dd>
              </div>
            ))}
          </dl>
        ) : (
          <p>Objective breakdown unavailable.</p>
        )}
      </details>
      {marginExplanation ? (
        <EconomicMargin margin={marginExplanation} />
      ) : (
        <p className="explanation-note">
          Economic margin evidence unavailable. Member rewards and conservative
          incremental margin are not supplied.
        </p>
      )}
    </section>
  );
}

function EvidenceTime({ value }: { value: z.infer<typeof timeSchema> }) {
  const iso = new Date(
    Number(value.seconds) * 1000 + value.nanos / 1e6,
  ).toISOString();
  return <time dateTime={iso}>{iso}</time>;
}

function EconomicMargin({
  margin,
}: {
  margin: NonNullable<z.infer<typeof explanationSchema>["marginExplanation"]>;
}) {
  return (
    <section>
      <h3>Modeled economic margin</h3>
      <p>
        Conservative margin {quantity.format(margin.conservativeMargin)} ·
        Hurdle {quantity.format(margin.marginHurdle)}. Currency and units not
        supplied.
      </p>
      <div className="explanation-scroll">
        <table aria-label="Economic margin terms">
          <thead>
            <tr>
              <th>Term</th>
              <th>Range</th>
              <th>Source</th>
            </tr>
          </thead>
          <tbody>
            {margin.terms.map((term, index) => (
              <tr key={index}>
                <th scope="row">{term.name}</th>
                <td className="mono">
                  {term.unavailable
                    ? "Unavailable"
                    : `${quantity.format(term.low)}–${quantity.format(term.high)}`}
                </td>
                <td>{term.source || "Not supplied"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

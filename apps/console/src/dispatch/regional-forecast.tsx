import { useState } from "react";
import { z } from "zod";
import { evidenceSchema, provenanceNames } from "../api/Provenance";
import type { PlanExplanationEvidence } from "../api/gen/gridos/v1/optimization_pb";

const timeSchema = evidenceSchema.shape.timestamp;
const valueSchema = z
  .object({
    value: z.number().finite(),
    lower: z.number().finite(),
    upper: z.number().finite(),
    valueKind: z.enum([
      "modeled_estimate",
      "forecast",
      "confirmed_public_forward",
      "simulated_forward",
    ]),
    provenance: z.number().int().min(1).max(5),
    issuedAt: timeSchema,
    modelVersion: z.string().min(1),
    featureVersion: z.string(),
    intervalCoverage: z.number().min(0).max(1).optional(),
  })
  .refine((value) => value.lower <= value.value && value.value <= value.upper);
const probabilitySchema = valueSchema.refine(
  (value) => value.lower >= 0 && value.upper <= 1,
);
const regionalSchema = z.object({
  regionalPrices: z.array(
    z.object({
      loadZone: z.string().min(1),
      intervalBeginTime: timeSchema,
      pricePerMwh: valueSchema,
    }),
  ),
  outageRisks: z.array(
    z.object({
      county: z.string().min(1),
      intervalBeginTime: timeSchema,
      probability: probabilitySchema,
    }),
  ),
  deviceAvailability: z.array(
    z.object({
      deviceId: z.string().min(1),
      intervalBeginTime: timeSchema,
      probability: probabilitySchema,
    }),
  ),
  unavailableSources: z.array(z.string().min(1)),
});
type ForecastRow = {
  subject: string;
  at: z.infer<typeof timeSchema>;
  value: z.infer<typeof valueSchema>;
};
type ForecastUnit = "USD/MWh" | "probability";
const percent = new Intl.NumberFormat("en-US", {
  style: "percent",
  maximumFractionDigits: 2,
});
function format(value: number, unit: ForecastUnit) {
  return unit === "probability" ? percent.format(value) : `${value} USD/MWh`;
}

export function RegionalForecasts({
  evidence,
}: {
  evidence: PlanExplanationEvidence | undefined;
}) {
  const [limit, setLimit] = useState(50);
  const result = regionalSchema.safeParse(evidence);
  if (!result.success)
    return (
      <p role={evidence ? "alert" : undefined}>
        {evidence
          ? "Regional forecast evidence is invalid."
          : "Frozen regional forecast evidence unavailable."}
      </p>
    );
  const values = result.data;
  const missing = [...new Set(values.unavailableSources)];
  return (
    <>
      <section aria-label="Dispatch window ranking">
        <h3>Dispatch window ranking unavailable</h3>
        <p>
          Interval-matched regional load is not supplied in the frozen input.
        </p>
        {!values.regionalPrices.length && (
          <p>Frozen regional prices unavailable.</p>
        )}
        {!values.outageRisks.length && (
          <p>Frozen outage probabilities unavailable.</p>
        )}
        <p>
          Window scoring requires matching price, regional load, outage
          probability, and feasible capacity. Historical context cannot
          substitute for missing event-window inputs.
        </p>
      </section>
      <ForecastSeries
        title="Frozen regional prices"
        subjectLabel="Load zone"
        unit="USD/MWh"
        rows={values.regionalPrices.map((row) => ({
          subject: row.loadZone,
          at: row.intervalBeginTime,
          value: row.pricePerMwh,
        }))}
      />
      <ForecastSeries
        title="Frozen outage risk"
        subjectLabel="County"
        unit="probability"
        rows={values.outageRisks.map((row) => ({
          subject: row.county,
          at: row.intervalBeginTime,
          value: row.probability,
        }))}
      />
      <ForecastSeries
        title="Frozen device availability"
        subjectLabel="Device"
        unit="probability"
        rows={values.deviceAvailability.map((row) => ({
          subject: row.deviceId,
          at: row.intervalBeginTime,
          value: row.probability,
        }))}
      />
      <section aria-label="Unavailable forecast sources">
        <h3>Unavailable forecast sources</h3>
        {missing.length ? (
          <>
            <p>
              {missing.length} unique source gaps. Missing forecasts are not
              zero-valued forecasts.
            </p>
            <ul>
              {missing.slice(0, limit).map((source) => (
                <li key={source}>{source}</li>
              ))}
            </ul>
            {limit < missing.length && (
              <button onClick={() => setLimit(limit + 50)}>
                Show 50 more source gaps
              </button>
            )}
          </>
        ) : (
          <p>No source gaps listed in the frozen input.</p>
        )}
      </section>
    </>
  );
}

function ForecastSeries({
  title,
  subjectLabel,
  unit,
  rows,
}: {
  title: string;
  subjectLabel: string;
  unit: ForecastUnit;
  rows: ForecastRow[];
}) {
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState("");
  const subjects = [...new Set(rows.map((row) => row.subject))].filter(
    (subject) => subject.toLowerCase().includes(search.trim().toLowerCase()),
  );
  const choices = subjects.slice(0, 50);
  const subject = choices.includes(selected) ? selected : choices[0];
  const intervals = rows.filter((row) => row.subject === subject);
  return (
    <section aria-label={title}>
      <h3>{title}</h3>
      {!rows.length ? (
        <p>No {title.toLowerCase()} returned. Evidence unavailable.</p>
      ) : (
        <>
          <p>
            {rows.length} frozen interval records. Classification and provenance
            are preserved per value.
          </p>
          <label className="forecast-filter">
            Find {subjectLabel.toLowerCase()}
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <p>
            Showing {choices.length} of {subjects.length} matching records.
          </p>
          {choices.length ? (
            <>
              <label className="forecast-filter">
                {subjectLabel}
                <select
                  value={subject}
                  onChange={(event) => setSelected(event.target.value)}
                >
                  {choices.map((value) => (
                    <option key={value}>{value}</option>
                  ))}
                </select>
              </label>
              <RegionalRanges rows={intervals} unit={unit} title={title} />
              <div
                className="explanation-scroll"
                role="region"
                aria-label={`${title} table`}
                tabIndex={0}
              >
                <table aria-label={title}>
                  <thead>
                    <tr>
                      <th>Interval begins (UTC)</th>
                      <th>Value</th>
                      <th>Range</th>
                      <th>Source and issue time</th>
                    </tr>
                  </thead>
                  <tbody>
                    {intervals.map((row, index) => (
                      <tr key={index}>
                        <td>
                          <ForecastTime value={row.at} />
                        </td>
                        <td>
                          {format(row.value.value, unit)}
                          <br />
                          {row.value.valueKind}
                        </td>
                        <td>
                          {format(row.value.lower, unit)} –{" "}
                          {format(row.value.upper, unit)}
                          <br />
                          {row.value.intervalCoverage === undefined
                            ? "Coverage not supplied"
                            : `${percent.format(row.value.intervalCoverage)} interval coverage`}
                        </td>
                        <td>
                          {
                            provenanceNames[
                              row.value
                                .provenance as keyof typeof provenanceNames
                            ]
                          }
                          <br />
                          <ForecastTime value={row.value.issuedAt} />
                          <br />
                          Model {row.value.modelVersion}
                          <br />
                          Features {row.value.featureVersion || "Not supplied"}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          ) : (
            <p>No matching forecast records.</p>
          )}
        </>
      )}
    </section>
  );
}

function RegionalRanges({
  rows,
  unit,
  title,
}: {
  rows: ForecastRow[];
  unit: ForecastUnit;
  title: string;
}) {
  const minimum = Math.min(0, ...rows.map((row) => row.value.lower));
  const maximum = Math.max(1, ...rows.map((row) => row.value.upper));
  const x = (value: number) =>
    24 + ((value - minimum) / (maximum - minimum)) * 272;
  return (
    <div className="forecast-ranges">
      <svg
        role="img"
        aria-label={`${title} and ranges`}
        viewBox={`0 0 320 ${rows.length * 40 + 32}`}
      >
        <text x="24" y="16">
          {format(minimum, unit)}
        </text>
        <text x="296" y="16" textAnchor="end">
          {format(maximum, unit)}
        </text>
        {rows.map((row, index) => (
          <g key={index}>
            <title>
              {format(row.value.value, unit)}; range{" "}
              {format(row.value.lower, unit)} – {format(row.value.upper, unit)}
            </title>
            <line
              x1={x(row.value.lower)}
              x2={x(row.value.upper)}
              y1={40 + index * 40}
              y2={40 + index * 40}
            />
            <circle cx={x(row.value.value)} cy={40 + index * 40} r="4" />
          </g>
        ))}
      </svg>
    </div>
  );
}

function ForecastTime({ value }: { value: z.infer<typeof timeSchema> }) {
  const iso = new Date(
    Number(value.seconds) * 1000 + value.nanos / 1e6,
  ).toISOString();
  return <time dateTime={iso}>{iso}</time>;
}

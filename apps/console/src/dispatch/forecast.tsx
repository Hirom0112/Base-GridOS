import { useState } from "react";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { z } from "zod";
import { evidenceSchema, provenanceNames } from "../api/Provenance";
import type { PlanExplanationEvidence } from "../api/gen/gridos/v1/optimization_pb";

const forecastSchema = z.object({
  siteLoadUnits: z.literal("kWh"),
  siteLoads: z.array(
    z.object({
      siteId: z.string().min(1),
      intervalBeginTime: evidenceSchema.shape.timestamp,
      loadKwh: z
        .object({
          value: z.number().nonnegative(),
          lower: z.number().nonnegative(),
          upper: z.number().nonnegative(),
          valueKind: z.literal("modeled_estimate"),
          provenance: z.number().int().min(1).max(5),
          issuedAt: evidenceSchema.shape.timestamp,
          modelVersion: z.string().min(1),
          featureVersion: z.string(),
        })
        .refine(
          (value) => value.lower <= value.value && value.value <= value.upper,
        ),
    }),
  ),
});

export function ForecastEvidence({
  evidence,
}: {
  evidence: PlanExplanationEvidence | undefined;
}) {
  const [selectedSite, setSelectedSite] = useState("");
  const parsed = forecastSchema.safeParse(evidence);
  const sites = parsed.success
    ? [...new Set(parsed.data.siteLoads.map((row) => row.siteId))]
    : [];
  const site = sites.includes(selectedSite) ? selectedSite : sites[0];
  return (
    <section aria-label="Forecast intervals">
      <h3>Frozen site load forecasts</h3>
      {!parsed.success ? (
        <p role={evidence ? "alert" : undefined}>
          {evidence
            ? "Frozen forecast evidence is invalid."
            : "Frozen forecast evidence unavailable."}
        </p>
      ) : (
        <>
          <p>
            MODELED · {sites.length} sites · {parsed.data.siteLoads.length}{" "}
            interval records. Forecasts belong to this plan’s frozen input; they
            are not measured consumption.
          </p>
          {sites.length ? (
            <>
              <label>
                Forecast site
                <select
                  value={site}
                  onChange={(event) => setSelectedSite(event.target.value)}
                >
                  {sites.map((id) => (
                    <option key={id}>{id}</option>
                  ))}
                </select>
              </label>
              <div
                className="explanation-scroll"
                role="region"
                aria-label="Site forecast table"
                tabIndex={0}
              >
                <table aria-label="Frozen site forecasts">
                  <thead>
                    <tr>
                      <th>Interval begins (UTC)</th>
                      <th>Expected load</th>
                      <th>Modeled range</th>
                      <th>Source and issue time</th>
                    </tr>
                  </thead>
                  <tbody>
                    {parsed.data.siteLoads
                      .filter((row) => row.siteId === site)
                      .map((row, index) => (
                        <tr key={index}>
                          <td>
                            {timestampDate(row.intervalBeginTime).toISOString()}
                          </td>
                          <td>{row.loadKwh.value} kWh</td>
                          <td>
                            {row.loadKwh.lower}–{row.loadKwh.upper} kWh
                          </td>
                          <td>
                            {
                              provenanceNames[
                                row.loadKwh
                                  .provenance as keyof typeof provenanceNames
                              ]
                            }
                            <br />
                            {timestampDate(row.loadKwh.issuedAt).toISOString()}
                            <br />
                            Model {row.loadKwh.modelVersion}
                            <br />
                            Features{" "}
                            {row.loadKwh.featureVersion || "Not supplied"}
                          </td>
                        </tr>
                      ))}
                  </tbody>
                </table>
              </div>
            </>
          ) : (
            <p>No frozen site forecasts returned.</p>
          )}
          <p>
            Interval end times, outage forecasts, and device availability
            forecasts are not supplied by this response.
          </p>
        </>
      )}
    </section>
  );
}

export function FallbackEvidence({
  evidence,
}: {
  evidence: PlanExplanationEvidence | undefined;
}) {
  return (
    <section aria-label="Solver fallback">
      <h3>Solver fallback</h3>
      {!evidence ? (
        <p>Fallback evidence unavailable.</p>
      ) : evidence.fallbackUsed ? (
        <p>
          Fallback used · {evidence.fallbackReason || "Reason not supplied"}
        </p>
      ) : (
        <p>No fallback recorded.</p>
      )}
    </section>
  );
}

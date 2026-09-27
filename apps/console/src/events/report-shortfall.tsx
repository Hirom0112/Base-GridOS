import { z } from "zod";

const number = z.number().finite();
const interval = {
  begin: z.iso.datetime({ offset: true }),
  end: z.iso.datetime({ offset: true }),
};
const chronological = (value: { begin: string; end: string }) =>
  Date.parse(value.end) > Date.parse(value.begin);
const plannedSchema = z.array(
  z
    .object({
      ...interval,
      requested_kw: number.nonnegative(),
      feasible_kw: number.nonnegative(),
      shortfall_kw: number.nonnegative(),
      reasons: z.array(z.string().min(1)).nullable(),
    })
    .refine(chronological),
);
const deliverySchema = z.array(
  z
    .discriminatedUnion("value_kind", [
      z.object({
        ...interval,
        requested_kwh: number.nonnegative(),
        coverage: number.gt(0).max(1),
        value_kind: z.literal("MEASURED"),
        measured_delivered_kwh: number,
        shortfall_kwh: number,
      }),
      z.object({
        ...interval,
        requested_kwh: number.nonnegative(),
        coverage: number.min(0).max(1),
        value_kind: z.literal("UNKNOWN"),
        measured_delivered_kwh: z.undefined().optional(),
        shortfall_kwh: z.undefined().optional(),
      }),
    ])
    .refine(chronological),
);

export function PlannedShortfall({ evidence }: { evidence: unknown }) {
  const result = plannedSchema.safeParse(evidence);
  return (
    <section aria-label="Planned shortfall evidence">
      <h4>Planned shortfall</h4>
      <p>Requested power versus feasible power in the approved plan.</p>
      {evidence == null ? (
        <p>Planned shortfall unavailable. No interval evidence supplied.</p>
      ) : !result.success ? (
        <p role="alert">Planned shortfall evidence is invalid.</p>
      ) : !result.data.length ? (
        <p>No planned shortfall intervals supplied.</p>
      ) : (
        <div
          className="report-scroll"
          role="region"
          aria-label="Planned shortfall table"
          tabIndex={0}
        >
          <table className="shortfall-table" aria-label="Planned shortfall">
            <caption>Scroll horizontally for all interval columns.</caption>
            <thead>
              <tr>
                <th>Interval</th>
                <th>Requested</th>
                <th>Feasible</th>
                <th>Shortfall</th>
                <th>Reasons</th>
              </tr>
            </thead>
            <tbody>
              {result.data.map((value, index) => (
                <tr key={index}>
                  <th scope="row">
                    <time dateTime={value.begin}>{value.begin}</time> –{" "}
                    <time dateTime={value.end}>{value.end}</time>
                  </th>
                  <td>{value.requested_kw.toFixed(3)} kW</td>
                  <td>{value.feasible_kw.toFixed(3)} kW</td>
                  <td>{value.shortfall_kw.toFixed(3)} kW</td>
                  <td>{value.reasons?.join(" · ") || "None supplied"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

export function DeliveryShortfall({ evidence }: { evidence: unknown }) {
  const result = deliverySchema.safeParse(evidence);
  return (
    <section aria-label="Delivery shortfall evidence">
      <h4>Delivery shortfall</h4>
      <p>
        Requested interval energy minus observed delivered energy. Negative
        values indicate observed delivery above the request. Partial coverage
        does not establish full delivery or a complete fleet shortfall.
      </p>
      {evidence == null ? (
        <p>Delivery shortfall unavailable. No interval evidence supplied.</p>
      ) : !result.success ? (
        <p role="alert">Delivery shortfall evidence is invalid.</p>
      ) : !result.data.length ? (
        <p>No delivery shortfall intervals supplied.</p>
      ) : (
        <div
          className="report-scroll"
          role="region"
          aria-label="Delivery shortfall table"
          tabIndex={0}
        >
          <table className="shortfall-table" aria-label="Delivery shortfall">
            <caption>Scroll horizontally for all interval columns.</caption>
            <thead>
              <tr>
                <th>Interval</th>
                <th>Requested</th>
                <th>Observed delivery</th>
                <th>Shortfall</th>
                <th>Coverage</th>
                <th>Evidence</th>
              </tr>
            </thead>
            <tbody>
              {result.data.map((value, index) => (
                <tr key={index}>
                  <th scope="row">
                    <time dateTime={value.begin}>{value.begin}</time> –{" "}
                    <time dateTime={value.end}>{value.end}</time>
                  </th>
                  <td>{value.requested_kwh.toFixed(3)} kWh</td>
                  <td>
                    {value.value_kind === "MEASURED"
                      ? `${value.measured_delivered_kwh.toFixed(3)} kWh`
                      : "Unavailable"}
                  </td>
                  <td>
                    {value.value_kind === "MEASURED"
                      ? `${value.shortfall_kwh.toFixed(3)} kWh`
                      : "Unavailable"}
                  </td>
                  <td>{(value.coverage * 100).toFixed(1)}%</td>
                  <td>{value.value_kind}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

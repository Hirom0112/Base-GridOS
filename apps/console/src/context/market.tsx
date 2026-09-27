import { useQuery } from "@tanstack/react-query";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { z } from "zod";
import { useSession } from "../api/auth";
import { evidenceSchema } from "../api/Provenance";
import type { GetMarketContextResponse } from "../api/gen/gridos/v1/api_pb";
import { ContextEvidence, sourceSchema } from "./source";

const priceSchema = z.object({
  intervalEnd: evidenceSchema.shape.timestamp,
  settlementPoint: z.literal("LZ_AEN"),
  usdPerMwh: z.number().finite(),
  source: sourceSchema,
});
const marketSchema = z.object({
  dayAheadPrices: z.array(priceSchema),
  realTimePrices: z.array(priceSchema),
  systemLoads: z.array(
    z.object({
      intervalEnd: evidenceSchema.shape.timestamp,
      weatherZone: z.literal("SOUTH_C"),
      mw: z.number().nonnegative(),
      source: sourceSchema,
    }),
  ),
});

export function MarketEvidence({ data }: { data: GetMarketContextResponse }) {
  if (!marketSchema.safeParse(data).success)
    return (
      <p role="alert">
        Market evidence is invalid. Do not rely on these values.
      </p>
    );
  const tables = [
    {
      name: "Day-ahead reference prices",
      rows: data.dayAheadPrices.map((row) => ({
        at: row.intervalEnd!,
        geography: row.settlementPoint,
        value: `${row.usdPerMwh} USD/MWh`,
        source: row.source,
      })),
    },
    {
      name: "Real-time reference prices",
      rows: data.realTimePrices.map((row) => ({
        at: row.intervalEnd!,
        geography: row.settlementPoint,
        value: `${row.usdPerMwh} USD/MWh`,
        source: row.source,
      })),
    },
    {
      name: "Reference system load",
      rows: data.systemLoads.map((row) => ({
        at: row.intervalEnd!,
        geography: row.weatherZone,
        value: `${row.mw} MW`,
        source: row.source,
      })),
    },
  ];
  return (
    <>
      <p className="boundary-note">
        Austin reference prices cover LZ_AEN; regional load covers SOUTH_C.
        Source dates may differ from the event window.
      </p>
      {tables.map(({ name, rows }) => (
        <section key={name}>
          <h4>{name}</h4>
          <p>{rows.length} source records · latest 12 intervals shown</p>
          <div
            className="table-scroll"
            role="region"
            aria-label={`${name} evidence`}
            tabIndex={0}
          >
            <table aria-label={name}>
              <thead>
                <tr>
                  <th>Interval ends (UTC)</th>
                  <th>Geography</th>
                  <th>Value</th>
                  <th>Source</th>
                </tr>
              </thead>
              <tbody>
                {rows
                  .sort(
                    (a, b) =>
                      Number(b.at.seconds - a.at.seconds) ||
                      b.at.nanos - a.at.nanos,
                  )
                  .slice(0, 12)
                  .map((row, index) => (
                    <tr key={index}>
                      <td>{timestampDate(row.at).toISOString()}</td>
                      <td>{row.geography}</td>
                      <td>{row.value}</td>
                      <td>
                        <ContextEvidence source={row.source} />
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
          {!rows.length && <p>No records returned by the source.</p>}
        </section>
      ))}
    </>
  );
}

export function MarketContext() {
  const { client, identity } = useSession();
  const query = useQuery({
    queryKey: ["market-context", "LZ_AEN", "SOUTH_C", identity.role],
    queryFn: ({ signal }) =>
      client.context.getMarketContext(
        { settlementPoint: "LZ_AEN", weatherZone: "SOUTH_C" },
        { signal },
      ),
  });
  return (
    <details>
      <summary>Inspect Austin markets · LZ_AEN and SOUTH_C</summary>
      {query.isPending && <p role="status">Loading reference markets…</p>}
      {query.isError && (
        <p role="alert">
          Reference market evidence unavailable.{" "}
          <button onClick={() => query.refetch()}>
            Retry reference markets
          </button>
        </p>
      )}
      {query.data && <MarketEvidence data={query.data} />}
    </details>
  );
}

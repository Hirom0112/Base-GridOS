import { ContextEvidence, sourceSchema } from "./source";
import { MarketContext } from "./market";
import "./regional.css";
import { useQuery } from "@tanstack/react-query";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { z } from "zod";
import { useSession } from "../api/auth";
import { evidenceSchema } from "../api/Provenance";
import type {
  GetWeatherContextResponse,
  GetOutageRiskResponse,
} from "../api/gen/gridos/v1/api_pb";

const forecastSchema = z.object({
  city: z.literal("austin"),
  beginTime: evidenceSchema.shape.timestamp,
  endTime: evidenceSchema.shape.timestamp,
  temperatureF: z.number().finite(),
  summary: z.string(),
  source: sourceSchema,
});
const weatherSchema = z.object({
  forecasts: z.array(forecastSchema),
  alerts: z.array(
    z.object({
      city: z.literal("austin"),
      event: z.string().min(1),
      severity: z.string().min(1),
      effectiveAt: evidenceSchema.shape.timestamp,
      expiresAt: evidenceSchema.shape.timestamp,
      source: sourceSchema,
    }),
  ),
});
const outageSchema = z.object({
  rates: z.array(
    z.object({
      county: z.literal("Travis"),
      month: z.string().regex(/^\d{4}-(0[1-9]|1[0-2])$/),
      rate: z.number().min(0).max(1),
      source: sourceSchema,
    }),
  ),
});

export function WeatherEvidence({ data }: { data: GetWeatherContextResponse }) {
  if (!weatherSchema.safeParse(data).success)
    return (
      <p role="alert">
        Weather evidence is invalid. Austin context cannot be established.
      </p>
    );
  return (
    <>
      <h4>Austin weather forecast</h4>
      <p className="history-gap">
        Source snapshot; check the forecast interval and issue time before
        planning.
      </p>
      {data.forecasts.length ? (
        <div
          className="table-scroll"
          role="region"
          aria-label="Austin weather forecasts"
          tabIndex={0}
        >
          <table aria-label="Austin weather forecasts">
            <thead>
              <tr>
                <th>Interval (UTC)</th>
                <th>Temperature</th>
                <th>Conditions</th>
                <th>Source</th>
              </tr>
            </thead>
            <tbody>
              {data.forecasts.map((forecast, index) => (
                <tr key={index}>
                  <td>
                    {timestampDate(forecast.beginTime!).toISOString()} –{" "}
                    {timestampDate(forecast.endTime!).toISOString()}
                  </td>
                  <td>{forecast.temperatureF} °F</td>
                  <td>{forecast.summary || "Not supplied"}</td>
                  <td>
                    <ContextEvidence source={forecast.source} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <p>No weather forecasts returned by the source.</p>
      )}
      <h4>Weather alerts</h4>
      {data.alerts.length ? (
        <ul>
          {data.alerts.map((alert, index) => (
            <li key={index}>
              <strong>
                {alert.event} · {alert.severity}
              </strong>
              <p>
                {timestampDate(alert.effectiveAt!).toISOString()} –{" "}
                {timestampDate(alert.expiresAt!).toISOString()}
              </p>
              <ContextEvidence source={alert.source} />
            </li>
          ))}
        </ul>
      ) : (
        <p>No weather alerts returned by the source.</p>
      )}
    </>
  );
}

export function OutageEvidence({ data }: { data: GetOutageRiskResponse }) {
  if (!outageSchema.safeParse(data).success)
    return (
      <p role="alert">
        Outage history is invalid. Travis County context cannot be established.
      </p>
    );
  return (
    <>
      <h4>Travis County outage history</h4>
      <p className="history-gap">
        Historical monthly rates are not a current outage probability.
      </p>
      {data.rates.length ? (
        <ul>
          {data.rates.map((rate, index) => (
            <li key={index}>
              <strong>{rate.month}</strong> ·{" "}
              <span>{(rate.rate * 100).toFixed(3)}% historical rate</span>
              <ContextEvidence source={rate.source} />
            </li>
          ))}
        </ul>
      ) : (
        <p>No historical outage rates returned by the source.</p>
      )}
    </>
  );
}

export function RegionalContext() {
  const { client, identity } = useSession();
  const weather = useQuery({
    queryKey: ["weather-context", "austin", identity.role],
    queryFn: ({ signal }) =>
      client.context.getWeatherContext({ city: "austin" }, { signal }),
  });
  const outages = useQuery({
    queryKey: ["outage-context", "Travis", identity.role],
    queryFn: ({ signal }) =>
      client.context.getOutageRisk({ county: "Travis" }, { signal }),
  });
  return (
    <section
      className="event-panel regional-context"
      aria-labelledby="regional-context-title"
    >
      <p className="eyebrow">Regional context</p>
      <h3 id="regional-context-title">Austin conditions</h3>
      <div className="boundary-note">
        Context snapshots do not establish event feasibility. Ranked dispatch
        windows require event-specific inputs.
      </div>
      <MarketContext />
      <details>
        <summary>Inspect weather and historical outage evidence</summary>
        {weather.isPending && <p role="status">Loading Austin weather…</p>}
        {weather.isError && (
          <p role="alert">
            Austin weather unavailable.{" "}
            <button onClick={() => weather.refetch()}>Retry weather</button>
          </p>
        )}
        {weather.data && <WeatherEvidence data={weather.data} />}
        {outages.isPending && (
          <p role="status">Loading Travis County history…</p>
        )}
        {outages.isError && (
          <p role="alert">
            Travis County history unavailable.{" "}
            <button onClick={() => outages.refetch()}>
              Retry outage history
            </button>
          </p>
        )}
        {outages.data && <OutageEvidence data={outages.data} />}
      </details>
    </section>
  );
}

import type { EventSample } from "./events-live";

export function ResponseChart({ samples }: { samples: EventSample[] }) {
  const begin = samples[0]?.time ?? 0;
  const end = samples.at(-1)?.time ?? begin;
  const values = samples.flatMap((sample) => [
    sample.sent,
    sample.acknowledged,
    sample.delivered ?? 0,
  ]);
  const low = Math.min(0, ...values);
  const high = Math.max(0.001, ...values);
  const x = (time: number) =>
    72 + (end === begin ? 0.5 : (time - begin) / (end - begin)) * 520;
  const y = (value: number) => 180 - ((value - low) / (high - low)) * 150;
  return (
    <svg className="response-chart" viewBox="0 0 640 220" aria-hidden="true">
      {[low, (high + low) / 2, high].map((value, index) => (
        <g key={index}>
          <line
            x1="72"
            x2="592"
            y1={y(value)}
            y2={y(value)}
            className="chart-grid"
          />
          <text x="64" y={y(value) + 4} textAnchor="end">
            {value.toFixed(3)}
          </text>
        </g>
      ))}
      <text x="72" y="210">
        {new Date(begin).toISOString().slice(11, 19)} UTC
      </text>
      <text x="592" y="210" textAnchor="end">
        {new Date(end).toISOString().slice(11, 19)} UTC
      </text>
      {(["sent", "acknowledged", "delivered"] as const).map((series) => (
        <g key={series} className={`series-${series}`}>
          {samples.map((sample, index) => {
            const value = sample[series];
            const previous = samples[index - 1];
            const previousValue = previous?.[series];
            if (value === null) return null;
            return (
              <g key={sample.order.toString()}>
                {previous &&
                  previousValue !== null &&
                  previousValue !== undefined && (
                    <line
                      x1={x(previous.time)}
                      y1={y(previousValue)}
                      x2={x(sample.time)}
                      y2={y(value)}
                    />
                  )}
                <circle cx={x(sample.time)} cy={y(value)} r="2.5" />
              </g>
            );
          })}
        </g>
      ))}
    </svg>
  );
}

import { z } from "zod";
import type {
  AggregateMetadata,
  FleetQuantityAggregate,
} from "./gen/gridos/v1/api_pb";
import { DataProvenance } from "./gen/gridos/v1/device_pb";

export const provenanceNames = {
  [DataProvenance.CONFIRMED_PUBLIC]: "CONFIRMED_PUBLIC",
  [DataProvenance.CONFIRMED_SANDBOX]: "CONFIRMED_SANDBOX",
  [DataProvenance.AUTHORIZED_OPERATIONAL]: "AUTHORIZED_OPERATIONAL",
  [DataProvenance.DERIVED]: "DERIVED",
  [DataProvenance.SIMULATED]: "SIMULATED",
} as const;

export const evidenceSchema = z.object({
  timestamp: z.object({
    seconds: z.bigint().min(-62135596800n).max(253402300799n),
    nanos: z.number().int().min(0).max(999999999),
  }),
  freshness: z.object({
    seconds: z.bigint().nonnegative(),
    nanos: z.number().int().min(0).max(999999999),
  }),
  provenanceMix: z
    .array(
      z.object({
        provenance: z.union([
          z.literal(1),
          z.literal(2),
          z.literal(3),
          z.literal(4),
          z.literal(5),
        ]),
        recordCount: z.bigint().positive(),
      }),
    )
    .min(1),
});

export function Evidence({
  metadata,
}: {
  metadata: AggregateMetadata | undefined;
}) {
  const result = evidenceSchema.safeParse(metadata);
  if (!result.success)
    return <span className="evidence-missing">Evidence unavailable</span>;
  const { timestamp, freshness, provenanceMix } = result.data;
  const observed = new Date(
    Number(timestamp.seconds) * 1000 + timestamp.nanos / 1000000,
  ).toISOString();
  return (
    <div className="aggregate-evidence">
      {provenanceMix.map((share) => (
        <span className="provenance-share" key={share.provenance}>
          <span>{provenanceNames[share.provenance]}</span> ·{" "}
          {share.recordCount.toLocaleString()} records
        </span>
      ))}
      <span>{age(Number(freshness.seconds) + freshness.nanos / 1e9)} old</span>
      <time dateTime={observed}>
        {observed.slice(0, 16).replace("T", " ")} UTC
      </time>
    </div>
  );
}

function age(seconds: number) {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)}h`;
  return `${Math.round(seconds / 86400)}d`;
}

export function Quantity({
  label,
  unit,
  aggregate,
}: {
  label: string;
  unit: string;
  aggregate: FleetQuantityAggregate | undefined;
}) {
  const valid =
    evidenceSchema.safeParse(aggregate?.metadata).success &&
    Number.isFinite(aggregate?.value);
  return (
    <div className="quantity">
      <span className="quantity-label">{label}</span>
      {valid && aggregate ? (
        <p className="quantity-value mono">
          {aggregate.value.toFixed(3)} <small>{unit}</small>
        </p>
      ) : (
        <p className="quantity-value" aria-label="Unavailable">
          —
        </p>
      )}
      <Evidence metadata={aggregate?.metadata} />
    </div>
  );
}

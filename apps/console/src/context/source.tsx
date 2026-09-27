import { z } from "zod";
import { evidenceSchema, provenanceNames } from "../api/Provenance";
import type { ContextSource } from "../api/gen/gridos/v1/api_pb";

export const sourceSchema = z.object({
  provenance: evidenceSchema.shape.provenanceMix.element.shape.provenance,
  asOf: evidenceSchema.shape.timestamp,
  freshness: evidenceSchema.shape.freshness,
});
export function ContextEvidence({
  source,
}: {
  source: ContextSource | undefined;
}) {
  const result = sourceSchema.safeParse(source);
  if (!result.success) return <span>Source evidence unavailable</span>;
  const { asOf, freshness, provenance } = result.data;
  const at = new Date(
    Number(asOf.seconds) * 1000 + asOf.nanos / 1e6,
  ).toISOString();
  return (
    <div className="aggregate-evidence">
      <span>{provenanceNames[provenance]}</span>
      <time dateTime={at}>{at}</time>
      <span>
        {Number(freshness.seconds) + freshness.nanos / 1e9}s old at observation
      </span>
    </div>
  );
}

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { useSession } from "../api/auth";
import { Evidence, evidenceSchema } from "../api/Provenance";
import type { DrilldownResponse, GeoNode } from "../api/gen/gridos/v1/geo_pb";

const hierarchySchema = z.object({
  nodes: z.array(
    z.object({
      id: z.string().min(1),
      parentId: z.string(),
      label: z.string().min(1),
      level: z.number().int().min(1).max(5),
      siteCount: z.bigint().nonnegative(),
      metadata: evidenceSchema,
    }),
  ),
  sites: z.array(z.object({})),
  metadata: evidenceSchema,
});

export function HierarchyEvidence({
  data,
  parentId,
  open,
}: {
  data: DrilldownResponse;
  parentId: string;
  open: (node: GeoNode) => void;
}) {
  const parsed = hierarchySchema.safeParse(data);
  if (
    !parsed.success ||
    parsed.data.nodes.some((node) => node.parentId !== parentId)
  )
    return <p role="alert">Hierarchy evidence is invalid.</p>;
  return (
    <>
      <ul className="hierarchy-nodes">
        {data.nodes.map((node) => (
          <li key={node.id}>
            <button className="secondary-button" onClick={() => open(node)}>
              {node.label}
            </button>
            <span>{node.siteCount.toLocaleString()} sites</span>
            <Evidence metadata={node.metadata} />
          </li>
        ))}
      </ul>
      {data.sites.length > 0 && (
        <p>
          {data.sites.length} authorized sites in this group · household details
          remain private.
        </p>
      )}
      {!data.nodes.length && !data.sites.length && (
        <p>No child regions or sites returned.</p>
      )}
    </>
  );
}

export function GeographicHierarchy() {
  const { client, identity } = useSession();
  const [path, setPath] = useState<GeoNode[]>([]);
  const parentId = path.at(-1)?.id ?? "";
  const query = useQuery({
    queryKey: ["geo-hierarchy", parentId, identity.role],
    queryFn: ({ signal }) => client.geo.drilldown({ parentId }, { signal }),
  });
  return (
    <section className="geographic-hierarchy" aria-label="Geographic hierarchy">
      <h3>Market to feeder</h3>
      <p>
        Hierarchy labels retain their source. Simulated groups do not establish
        operational service boundaries.
      </p>
      <nav aria-label="Geographic path">
        <button className="text-button" onClick={() => setPath([])}>
          All markets
        </button>
        {path.map((node, index) => (
          <button
            className="text-button"
            key={node.id}
            onClick={() => setPath(path.slice(0, index + 1))}
          >
            {node.label}
          </button>
        ))}
      </nav>
      {query.isPending && <p role="status">Loading geographic hierarchy…</p>}
      {query.isError && (
        <p role="alert">
          Hierarchy unavailable.{" "}
          <button onClick={() => query.refetch()}>Retry hierarchy</button>
        </p>
      )}
      {query.data && (
        <HierarchyEvidence
          data={query.data}
          parentId={parentId}
          open={(node) => setPath([...path, node])}
        />
      )}
    </section>
  );
}

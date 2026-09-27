import { cellToBoundary } from "h3-js";
import type { H3SiteAggregate } from "../api/gen/gridos/v1/api_pb";
import { projectCells } from "../fleet/scene";

export function mapFeatures(cells: H3SiteAggregate[]) {
  projectCells(cells);
  return {
    type: "FeatureCollection" as const,
    features: cells.map((cell) => ({
      type: "Feature" as const,
      id: cell.h3Cell,
      properties: {
        id: cell.h3Cell,
        sites: Number(cell.siteCount),
        capacity: cell.installedMw!.value,
      },
      geometry: {
        type: "Polygon" as const,
        coordinates: [cellToBoundary(cell.h3Cell, true)],
      },
    })),
  };
}

export type MapFeatures = ReturnType<typeof mapFeatures>;
export type MapMeasure = "capacity" | "sites";

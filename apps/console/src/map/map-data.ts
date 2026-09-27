import {
  cellArea,
  cellToBoundary,
  getResolution,
  isValidCell,
  UNITS,
} from "h3-js";
import { z } from "zod";
import { evidenceSchema } from "../api/Provenance";
import type { GeoCell } from "../api/gen/gridos/v1/geo_pb";

const count = z.bigint().nonnegative().max(BigInt(Number.MAX_SAFE_INTEGER));
const cellsSchema = z
  .array(
    z.object({
      h3Cell: z.string().refine(isValidCell),
      siteCount: count,
      installedMw: z.number().nonnegative(),
      installedMwh: z.number().nonnegative(),
      dispatchableMw: z.number().nonnegative(),
      reservedMwh: z.number().nonnegative(),
      connectedCount: count,
      activeDispatchCount: count,
      socLowCount: count,
      socMediumCount: count,
      socHighCount: count,
      socUnknownCount: count,
      metadata: evidenceSchema,
    }),
  )
  .max(10000);

export const mapMeasureSchema = z.enum([
  "capacity",
  "sites",
  "dispatchable",
  "reserved",
  "connected",
  "active",
  "low",
  "medium",
  "high",
  "unknown",
]);
export type MapMeasure = z.infer<typeof mapMeasureSchema>;
export const mapMeasureLabels: Record<MapMeasure, string> = {
  capacity: "Installed capacity · MW",
  sites: "Site density · count",
  dispatchable: "Dispatchable power · MW",
  reserved: "Backup reserve · MWh",
  connected: "Connected sites · count",
  active: "Active dispatch sites · count",
  low: "Low state of charge · sites",
  medium: "Medium state of charge · sites",
  high: "High state of charge · sites",
  unknown: "Unknown state of charge · sites",
};

export function mapFeatures(input: GeoCell[]) {
  const cells = cellsSchema.parse(input);
  const finest = Math.max(...cells.map((cell) => getResolution(cell.h3Cell)));
  return {
    type: "FeatureCollection" as const,
    features: cells.map((cell) => ({
      type: "Feature" as const,
      id: cell.h3Cell,
      properties: {
        id: cell.h3Cell,
        sites: Number(cell.siteCount),
        capacity: cell.installedMw,
        dispatchable: cell.dispatchableMw,
        reserved: cell.reservedMwh,
        connected: Number(cell.connectedCount),
        active: Number(cell.activeDispatchCount),
        low: Number(cell.socLowCount),
        medium: Number(cell.socMediumCount),
        high: Number(cell.socHighCount),
        unknown: Number(cell.socUnknownCount),
        area: cellArea(cell.h3Cell, UNITS.km2),
        coarse: getResolution(cell.h3Cell) < finest,
      },
      geometry: {
        type: "Polygon" as const,
        coordinates: [cellToBoundary(cell.h3Cell, true)],
      },
    })),
  };
}
export type MapFeatures = ReturnType<typeof mapFeatures>;

import { cellToBoundary, cellToLatLng, isValidCell } from "h3-js";
import { z } from "zod";
import { evidenceSchema } from "../api/Provenance";
import type { H3SiteAggregate } from "../api/gen/gridos/v1/api_pb";

const cellSchema = z.object({
  h3Cell: z.string().refine(isValidCell),
  siteCount: z.bigint().positive().max(BigInt(Number.MAX_SAFE_INTEGER)),
  installedMw: z.object({
    value: z.number().nonnegative(),
    metadata: evidenceSchema,
  }),
});

export function projectCells(input: H3SiteAggregate[]) {
  const cells = z.array(cellSchema).max(10000).parse(input);
  if (!cells.length) return [];
  const positions = cells.map((cell) => cellToLatLng(cell.h3Cell));
  const center = positions.reduce(
    ([lat, lon], point) => [
      lat + point[0] / cells.length,
      lon + point[1] / cells.length,
    ],
    [0, 0] as [number, number],
  );
  const project = ([lat, lon]: number[]): [number, number] => [
    ((lon ?? 0) - center[1]) * Math.cos((center[0] * Math.PI) / 180) * 111.32,
    -((lat ?? 0) - center[0]) * 111.32,
  ];
  const maximum = Math.max(...cells.map((cell) => Number(cell.siteCount)));
  return cells.map((cell, index) => ({
    id: cell.h3Cell,
    position: project(positions[index] ?? center),
    boundary: cellToBoundary(cell.h3Cell).map(project),
    footprint: Math.sqrt(Number(cell.siteCount) / maximum) * 0.9,
    height: cell.installedMw.value * 18,
  }));
}

export type GridCell = ReturnType<typeof projectCells>[number];

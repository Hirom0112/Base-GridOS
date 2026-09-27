import { cellToBoundary, cellToLatLng, isValidCell } from "h3-js";
import { z } from "zod";
import { evidenceSchema } from "../api/Provenance";
import type {
  H3EventPowerAggregate,
  H3SiteAggregate,
} from "../api/gen/gridos/v1/api_pb";

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
    color: 0xa6b9ae,
    value: cell.installedMw.value as number | null,
    position: project(positions[index] ?? center),
    boundary: cellToBoundary(cell.h3Cell).map(project),
    footprint: Math.sqrt(Number(cell.siteCount) / maximum) * 0.9,
    height: cell.installedMw.value * 18,
  }));
}

export type GridCell = ReturnType<typeof projectCells>[number];

export type ResponseMeasure = "sent" | "acknowledged" | "delivered";

export function projectResponse(
  cells: GridCell[],
  response: { state: number; h3: H3EventPowerAggregate[] },
  measure: ResponseMeasure,
): GridCell[] {
  const colors = {
    sent: 0xf0f7f2,
    acknowledged: 0x6db8ff,
    delivered: 0x66f2a4,
  };
  const readings = new Map(
    response.h3.map((cell) => [cell.h3Cell, cell.power]),
  );
  return cells.map((cell) => {
    const power = readings.get(cell.id);
    let value: number | null = null;
    if (response.state >= 6 && power) {
      if (measure === "sent") value = power.sentMw;
      if (measure === "acknowledged") value = power.acknowledgedMw;
      if (measure === "delivered" && [1, 5].includes(power.deliveredState))
        value = power.deliveredMw;
    }
    return {
      ...cell,
      value,
      height: value === null ? 0 : Math.abs(value) * 18,
      color: value === null ? 0x66736c : colors[measure],
    };
  });
}

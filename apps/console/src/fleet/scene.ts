import {
  cellArea,
  cellsToMultiPolygon,
  cellToBoundary,
  cellToLatLng,
  getHexagonAreaAvg,
  getResolution,
  gridDisk,
  isValidCell,
  latLngToCell,
  UNITS,
} from "h3-js";
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
  const finest = Math.max(...cells.map((cell) => getResolution(cell.h3Cell)));
  const origin = fieldOrigin(
    cells
      .map((cell) => cell.h3Cell)
      .filter((id) => getResolution(id) === finest),
  );
  const project = projector(origin);
  const measured = cells.map((cell) => {
    const area = cellArea(cell.h3Cell, UNITS.km2);
    const position = project(cellToLatLng(cell.h3Cell));
    return { cell, area, position, density: cell.installedMw.value / area };
  });
  const extent = Math.max(
    20,
    ...measured
      .filter(({ cell }) => getResolution(cell.h3Cell) === finest)
      .map(({ position }) => Math.hypot(...position) * 2),
  );
  const densest = Math.max(...measured.map(({ density }) => density));
  const scale = densest > 0 ? (extent * 0.16) / densest : 0;
  return measured.map(({ cell, area, position, density }) => ({
    id: cell.h3Cell,
    origin,
    color: 0xa6b9ae,
    response: null as ResponseMeasure | null,
    value: cell.installedMw.value as number | null,
    position,
    boundary: cellToBoundary(cell.h3Cell).map(project),
    coarse: getResolution(cell.h3Cell) < finest,
    area,
    scale,
    footprint: 0.9,
    height: density * scale,
  }));
}

function fieldOrigin(ids: string[]): [number, number] {
  return ids
    .map((id) => cellToLatLng(id))
    .reduce(
      ([lat, lon], point) => [
        lat + point[0] / ids.length,
        lon + point[1] / ids.length,
      ],
      [0, 0] as [number, number],
    );
}

function projector([originLat, originLon]: [number, number]) {
  return ([lat, lon]: number[]): [number, number] => [
    ((lon ?? 0) - originLon) * Math.cos((originLat * Math.PI) / 180) * 111.32,
    -((lat ?? 0) - originLat) * 111.32,
  ];
}

const imageryBounds = {
  north: 31.0,
  south: 29.6,
  west: -98.55,
  east: -96.95,
};

const places = [
  ["Austin", 30.2672, -97.7431],
  ["Round Rock", 30.5083, -97.6789],
  ["Georgetown", 30.6333, -97.677],
  ["Pflugerville", 30.4394, -97.62],
  ["Cedar Park", 30.5052, -97.8203],
  ["Leander", 30.5788, -97.8531],
  ["Lakeway", 30.3632, -97.9795],
  ["Dripping Springs", 30.1902, -98.0867],
  ["Manor", 30.3405, -97.5567],
  ["Bastrop", 30.1105, -97.3153],
  ["Buda", 30.0852, -97.8403],
  ["Kyle", 29.9891, -97.8772],
  ["San Marcos", 29.8833, -97.9414],
  ["San Antonio", 29.4241, -98.4936],
] as const;

export function fieldGround(cells: GridCell[]) {
  const first = cells[0];
  if (!first)
    return {
      lattice: [],
      outline: [],
      places: [],
      radius: 0,
      imagery: { west: 0, east: 0, north: 0, south: 0 },
    };
  const project = projector(first.origin);
  const [west, north] = project([imageryBounds.north, imageryBounds.west]);
  const [east, south] = project([imageryBounds.south, imageryBounds.east]);
  const finest = Math.max(...cells.map((cell) => getResolution(cell.id)));
  const resolution = Math.min(finest, 7);
  const spacing = Math.sqrt(
    (2 * getHexagonAreaAvg(resolution, UNITS.km2)) / Math.sqrt(3),
  );
  const reach = Math.max(...cells.map((cell) => Math.hypot(...cell.position)));
  const rings = Math.min(40, Math.ceil((reach + 20) / spacing));
  const radius = rings * spacing;
  const lattice = gridDisk(
    latLngToCell(first.origin[0], first.origin[1], resolution),
    rings,
  ).map((id) => cellToBoundary(id).map(project));
  const outline = cellsToMultiPolygon(
    cells.filter((cell) => !cell.coarse).map((cell) => cell.id),
  ).map(([outer]) => (outer ?? []).map(project));
  return {
    lattice,
    outline,
    radius,
    imagery: { west, east, north, south },
    places: places.flatMap(([name, lat, lon]) => {
      const position = project([lat, lon]);
      return Math.hypot(...position) <= radius * 0.9
        ? [{ name, position }]
        : [];
    }),
  };
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
      response: value !== null && value !== 0 ? measure : null,
      height: value === null ? 0 : (Math.abs(value) / cell.area) * cell.scale,
      color: value === null ? 0x66736c : colors[measure],
    };
  });
}

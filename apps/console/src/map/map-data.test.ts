import { create } from "@bufbuild/protobuf";
import { expect, test } from "vitest";
import { GeoCellSchema } from "../api/gen/gridos/v1/geo_pb";
import { mapFeatures } from "./map-data";

const cell = create(GeoCellSchema, {
  h3Cell: "87489d884ffffff",
  siteCount: 12n,
  installedMw: 0.1,
  installedMwh: 0.3,
  dispatchableMw: 0.04,
  reservedMwh: 0.1,
  connectedCount: 8n,
  activeDispatchCount: 2n,
  socLowCount: 2n,
  socMediumCount: 3n,
  socHighCount: 4n,
  socUnknownCount: 3n,
  metadata: {
    timestamp: { seconds: 1786575300n },
    freshness: {},
    provenanceMix: [{ provenance: 5, recordCount: 12n }],
  },
});

test("map uses closed geographic H3 boundaries and only aggregate properties", () => {
  const collection = mapFeatures([cell]);
  const feature = collection.features[0];
  expect(feature?.properties).toEqual({
    id: cell.h3Cell,
    sites: 12,
    capacity: 0.1,
    dispatchable: 0.04,
    reserved: 0.1,
    connected: 8,
    active: 2,
    low: 2,
    medium: 3,
    high: 4,
    unknown: 3,
  });
  const ring = feature?.geometry.coordinates[0];
  expect(ring).toHaveLength(7);
  expect(ring?.[0]).toEqual(ring?.at(-1));
  expect(ring?.[0]?.[0]).toBeLessThan(-90);
  expect(ring?.[0]?.[1]).toBeGreaterThan(25);
  expect(mapFeatures([]).features).toEqual([]);
});

test("map rejects invalid evidence instead of displaying unproven capacity", () => {
  expect(() => mapFeatures([cell])).not.toThrow();
  expect(() => mapFeatures([{ ...cell, h3Cell: "invalid" }])).toThrow();
  expect(() => mapFeatures([{ ...cell, metadata: undefined }])).toThrow();
});

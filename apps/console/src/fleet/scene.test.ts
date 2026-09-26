import { create } from "@bufbuild/protobuf";
import { describe, expect, test } from "vitest";
import { H3SiteAggregateSchema } from "../api/gen/gridos/v1/api_pb";
import { projectCells } from "./scene";

const metadata = {
  timestamp: { seconds: 1786575300n, nanos: 0 },
  freshness: { seconds: 0n, nanos: 0 },
  provenanceMix: [{ provenance: 5, recordCount: 12n }],
};
const cell = create(H3SiteAggregateSchema, {
  h3Cell: "87489d884ffffff",
  siteCount: 12n,
  installedMw: { value: 0.1, metadata },
});

describe("scene", () => {
  test("projects contract locations with capacity height and finite boundaries", () => {
    const result = projectCells([cell]);
    expect(result).toHaveLength(1);
    expect(result[0]?.height).toBeGreaterThan(0);
    expect(result[0]?.boundary).toHaveLength(6);
    expect(result[0]?.boundary.flat().every(Number.isFinite)).toBe(true);
  });
  test("rejects invalid geographic identifiers and missing operational evidence", () => {
    expect(() => projectCells([cell])).not.toThrow();
    expect(() => projectCells([{ ...cell, h3Cell: "not-h3" }])).toThrow();
    expect(() => projectCells([{ ...cell, installedMw: undefined }])).toThrow();
    expect(() => projectCells([{ ...cell, siteCount: 0n }])).toThrow();
  });
});

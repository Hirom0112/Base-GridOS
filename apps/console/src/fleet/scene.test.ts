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

test("response projection never distributes fleet totals or invents missing cells", async () => {
  const { projectResponse } = await import("./scene");
  const { H3EventPowerAggregateSchema } =
    await import("../api/gen/gridos/v1/api_pb");
  const response = {
    state: 6,
    h3: [
      create(H3EventPowerAggregateSchema, {
        h3Cell: cell.h3Cell,
        power: {
          sentMw: 0.1,
          acknowledgedMw: 0.08,
          deliveredMw: 0.06,
          deliveredState: 1,
        },
      }),
    ],
  };
  const cells = projectCells([cell]);
  expect(projectResponse(cells, response, "sent")[0]?.value).toBe(0.1);
  expect(projectResponse(cells, response, "sent")[0]?.response).toBe("sent");
  expect(
    projectResponse(cells, { ...response, state: 4 }, "sent")[0]?.response,
  ).toBeNull();
  expect(cells[0]?.response).toBeNull();
  expect(projectResponse(cells, response, "acknowledged")[0]?.value).toBe(0.08);
  expect(projectResponse(cells, response, "delivered")[0]?.value).toBe(0.06);
  expect(
    projectResponse(cells, { ...response, h3: [] }, "delivered")[0]?.value,
  ).toBeNull();
  expect(
    projectResponse(cells, { ...response, state: 4 }, "sent")[0]?.value,
  ).toBeNull();
});

test("unknown measured cell power remains a gap even with a numerical payload", async () => {
  const { projectResponse } = await import("./scene");
  const { H3EventPowerAggregateSchema } =
    await import("../api/gen/gridos/v1/api_pb");
  const response = {
    state: 6,
    h3: [
      create(H3EventPowerAggregateSchema, {
        h3Cell: cell.h3Cell,
        power: { deliveredMw: 0.06, deliveredState: 4 },
      }),
    ],
  };
  expect(
    projectResponse(projectCells([cell]), response, "delivered")[0]?.value,
  ).toBeNull();
});

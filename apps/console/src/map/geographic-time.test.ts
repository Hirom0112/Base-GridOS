import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { expect, test } from "vitest";
import { ListCellsResponseSchema } from "../api/gen/gridos/v1/geo_pb";
import { verifyGeographicTime } from "./geographic-time";

test("historical geography must acknowledge the exact requested timestamp", () => {
  const requested = timestampFromDate(new Date("2026-09-27T12:00:00Z"));
  const response = create(ListCellsResponseSchema, { asOf: requested });
  expect(() => verifyGeographicTime(response, requested)).not.toThrow();
  expect(() => verifyGeographicTime(response, undefined)).not.toThrow();
  expect(() =>
    verifyGeographicTime({ asOf: { ...requested, nanos: 1 } }, requested),
  ).toThrow("requested replay time");
  expect(() => verifyGeographicTime({}, requested)).toThrow(
    "requested replay time",
  );
});

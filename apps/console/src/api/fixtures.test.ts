import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { fromJsonString, toJsonString } from "@bufbuild/protobuf";
import { expect, test } from "vitest";
import {
  GetFleetSummaryResponseSchema,
  ListSitesResponseSchema,
} from "./gen/gridos/v1/api_pb";
import { CommandIntentSchema } from "./gen/gridos/v1/dispatch_pb";

test.each([
  ["FleetService/GetFleetSummary", GetFleetSummaryResponseSchema],
  ["FleetService/ListSites", ListSitesResponseSchema],
] as const)(
  "fixtures decode %s through their generated contract",
  (path, schema) => {
    const json = readFileSync(
      resolve(process.cwd(), `../../testdata/fixtures/api/${path}.json`),
      "utf8",
    );
    expect(() => fromJsonString(schema, json)).not.toThrow();
  },
);

test("contract CommandIntent round-trips canonical JSON", () => {
  const json = readFileSync(
    resolve(
      process.cwd(),
      "../../testdata/fixtures/contracts/command_intent.json",
    ),
    "utf8",
  );
  const command = fromJsonString(CommandIntentSchema, json);
  expect(JSON.parse(toJsonString(CommandIntentSchema, command))).toEqual(
    JSON.parse(json),
  );
  expect(toJsonString(CommandIntentSchema, command)).toBe(
    JSON.stringify(JSON.parse(json)),
  );
});

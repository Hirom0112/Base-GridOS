import { readFile } from "node:fs/promises";
import { create, fromJsonString, toJsonString } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { expect, test, type Page } from "@playwright/test";
import { ReplayEventResponseSchema } from "../src/api/gen/gridos/v1/api_pb";
import { ListEventCommandsResponseSchema } from "../src/api/gen/gridos/v1/events_pb";
import {
  DrilldownResponseSchema,
  ListCellsRequestSchema,
  ListCellsResponseSchema,
  DrilldownRequestSchema,
} from "../src/api/gen/gridos/v1/geo_pb";
import { recordedApi } from "./recorded-api";

const eventId = "event_austin_wave2_live_0002";
const times = ["2026-09-27T12:00:01Z", "2026-09-27T12:00:02Z"];
async function replayApi(page: Page) {
  await recordedApi(page);
  const response = create(ReplayEventResponseSchema, {
    seed: 42n,
    inputSnapshotId: "input-1",
    eligibilitySnapshotId: "eligible-1",
    policyVersion: "policy-1",
    solverVersion: "solver-1",
    codeVersion: "code-1",
    fleetSha256: "a".repeat(64),
    diffStatus: "IDENTICAL",
    updates: times.map((at, index) => ({
      sequence: BigInt(index + 1),
      occurredAt: timestampFromDate(new Date(at)),
      actorId: "workflow",
      action: `ACTION_${index + 1}`,
      state: index + 1,
      reason: "Recorded",
    })),
  });
  await page.route("**/gridos.v1.ReplayService/ReplayEvent", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: toJsonString(ReplayEventResponseSchema, response),
    }),
  );
}

test("one replay clock drives the field, map and geographic hierarchy", async ({
  page,
}, testInfo) => {
  await replayApi(page);
  const requested: string[] = [];
  await page.route("**/gridos.v1.EventsService/ListEventCommands", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: toJsonString(
        ListEventCommandsResponseSchema,
        create(ListEventCommandsResponseSchema, {
          verificationIntervals: times.map((at, index) => ({
            beginTime: timestampFromDate(new Date("2026-09-27T12:00:00Z")),
            endTime: timestampFromDate(new Date(at)),
            requestedKw: 4,
            commandedKw: 3,
            deliveredKw: index === 0 ? 2.25 : 8.5,
            trackingErrorKw: index === 0 ? -1.75 : 4.5,
            confidence: 0.9,
            measurementBoundary: "METER_NET_EXPORT",
            baselineMethod: "DIRECT",
            valueKind: "MEASURED",
          })),
        }),
      ),
    }),
  );
  const hierarchy: string[] = [];
  await page.route("**/gridos.v1.GeoService/ListCells", async (route) => {
    const input = fromJsonString(
      ListCellsRequestSchema,
      route.request().postData()!,
    );
    requested.push(input.asOf?.seconds.toString() ?? "current");
    const response = fromJsonString(
      ListCellsResponseSchema,
      await readFile(
        "../../testdata/fixtures/api/GeoService/ListCells.json",
        "utf8",
      ),
    );
    if (input.asOf) {
      response.asOf = input.asOf;
      for (const cell of response.cells) {
        cell.asOf = input.asOf;
        cell.installedMw = input.asOf.seconds % 2n === 0n ? 3 : 2;
        cell.metadata!.timestamp = input.asOf;
      }
    }
    await route.fulfill({
      contentType: "application/json",
      body: toJsonString(ListCellsResponseSchema, response),
    });
  });
  await page.route("**/gridos.v1.GeoService/Drilldown", async (route) => {
    const input = fromJsonString(
      DrilldownRequestSchema,
      route.request().postData()!,
    );
    hierarchy.push(input.asOf?.seconds.toString() ?? "current");
    const response = create(DrilldownResponseSchema, {
      asOf: input.asOf,
      metadata: {
        timestamp: input.asOf ?? timestampFromDate(new Date()),
        freshness: { seconds: 0n },
        provenanceMix: [{ provenance: 5, recordCount: 1n }],
      },
    });
    await route.fulfill({
      contentType: "application/json",
      body: toJsonString(DrilldownResponseSchema, response),
    });
  });
  await page.goto(`/events/${eventId}/report`);
  await page.getByRole("button", { name: "Replay event", exact: true }).click();
  const field = page.getByRole("region", { name: "Living Grid geography" });
  await expect(field).toContainText("2.000");
  await expect(
    page.getByRole("table", { name: "Replay interval power" }),
  ).toContainText("2.25 kW");
  await expect(
    page.getByRole("table", { name: "Replay interval power" }),
  ).not.toContainText("8.5 kW");
  await page.getByRole("slider", { name: "Replay position" }).fill("1");
  await expect(field).toContainText("3.000");
  await expect(
    page.getByRole("table", { name: "Replay interval power" }),
  ).toContainText("8.5 kW");
  await page
    .getByRole("region", { name: "Replay interval measurements" })
    .screenshot({ path: testInfo.outputPath("replay-interval-chart.png") });
  const first = timestampFromDate(new Date(times[0]!)).seconds.toString();
  const last = timestampFromDate(new Date(times[1]!)).seconds.toString();
  expect(requested).toContain(first);
  expect(requested).toContain(last);
  await expect(page.getByLabel("Replay time")).toContainText("12:00:02");
  await page.getByRole("link", { name: "Explore map" }).click();
  await expect(
    page.getByRole("region", { name: "Geographic hierarchy" }),
  ).toContainText("No child regions");
  expect(hierarchy).toContain(last);
  await expect(
    page.getByRole("region", { name: "Historical geography clock" }),
  ).toContainText("12:00:02");
  await page
    .getByRole("button", { name: "Return to current geography" })
    .click();
  await expect.poll(() => hierarchy.at(-1)).toBe("current");
  await expect.poll(() => requested.at(-1)).toBe("current");
  await page.screenshot({
    path: testInfo.outputPath("replay-map-clock.png"),
    fullPage: true,
  });
});

test("the historical field rejects a current response instead of displaying it", async ({
  page,
}) => {
  await replayApi(page);
  await page.goto(`/events/${eventId}/report`);
  await page.getByRole("button", { name: "Replay event", exact: true }).click();
  await expect(
    page
      .getByRole("alert")
      .filter({ hasText: "Historical geography unavailable" }),
  ).toContainText("requested replay time");
  await expect(
    page.getByRole("region", { name: "Living Grid geography" }),
  ).toContainText("0 cells");
});

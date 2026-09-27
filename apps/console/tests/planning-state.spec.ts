import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { fromJsonString, toJsonString } from "@bufbuild/protobuf";
import { expect, test } from "@playwright/test";
import { GetEventResponseSchema } from "../src/api/gen/gridos/v1/api_pb";
import { DispatchEventState } from "../src/api/gen/gridos/v1/dispatch_pb";
import { recordedApi } from "./recorded-api";

test("queued planning has no invented plan version or premature approval", async ({
  page,
}, testInfo) => {
  await recordedApi(page);
  const recorded = await readFile(
    resolve(
      process.cwd(),
      "../../testdata/fixtures/api/DispatchService/GetEvent.json",
    ),
    "utf8",
  );
  const queued = fromJsonString(GetEventResponseSchema, recorded);
  if (!queued.event) throw new Error("Recorded event is required");
  queued.event.state = DispatchEventState.REQUESTED;
  queued.event.planVersion = 0n;
  queued.report = undefined;
  queued.exclusions = [];
  let validated = false;
  await page.route(
    "**/rpc/gridos.v1.DispatchService/GetEvent",
    async (route) => {
      await route.fulfill({
        contentType: "application/json",
        body: validated
          ? recorded
          : toJsonString(GetEventResponseSchema, queued),
      });
    },
  );
  await page.goto("/dispatch/event_austin_wave2_live_0002");
  await page.getByLabel("Demo role").selectOption("approver");
  await expect(page.locator(".event-identifiers")).toContainText(
    "Plan pending",
  );
  await expect(
    page
      .locator(".event-panel")
      .getByRole("status")
      .filter({ hasText: "Planning is queued" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Review approval" }),
  ).toHaveCount(0);
  await page
    .locator(".event-panel")
    .screenshot({ path: testInfo.outputPath("planning-queued.png") });
  validated = true;
  await expect(
    page.getByRole("heading", { name: "Safety validated" }),
  ).toBeVisible({ timeout: 5000 });
  await expect(page.locator(".event-identifiers")).toContainText("Plan v1");
  await expect(
    page.getByRole("button", { name: "Review approval" }),
  ).toBeVisible();
});

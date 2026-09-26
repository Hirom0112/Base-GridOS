import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { fromJsonString, toJsonString } from "@bufbuild/protobuf";
import { expect, test } from "@playwright/test";
import { ListSitesResponseSchema } from "../src/api/gen/gridos/v1/api_pb";
import { recordedApi } from "./recorded-api";

test("fleet polling updates selected evidence without replacing the canvas", async ({
  page,
}) => {
  await recordedApi(page);
  const response = fromJsonString(
    ListSitesResponseSchema,
    await readFile(
      resolve(
        process.cwd(),
        "../../testdata/fixtures/api/FleetService/ListSites.json",
      ),
      "utf8",
    ),
  );
  const location = response.sites[0]?.location;
  if (location?.case !== "aggregate" || !location.value.installedMw)
    throw new Error("Recorded geographic capacity is required");
  const cell = location.value;
  let requests = 0;
  await page.route("**/rpc/gridos.v1.FleetService/ListSites", async (route) => {
    requests++;
    await route.fulfill({
      contentType: "application/json",
      body: toJsonString(ListSitesResponseSchema, response),
    });
  });
  await page.goto("/fleet");
  await expect(page.locator("canvas")).toHaveCount(1);
  const canvas = await page.locator("canvas").elementHandle();
  await page.getByText("Inspect the geographic data").click();
  await page.getByRole("button", { name: cell.h3Cell, exact: true }).click();
  const focus = page
    .locator(".evidence-card")
    .filter({ has: page.getByRole("heading", { name: cell.h3Cell }) });
  await expect(focus).toContainText(
    cell.installedMw?.value.toFixed(3) ?? "missing capacity",
  );
  if (!cell.installedMw) throw new Error("Recorded capacity is required");
  cell.installedMw.value = 0.333;
  await expect(focus).toContainText("0.333", { timeout: 7000 });
  expect(requests).toBeGreaterThan(1);
  expect(
    await canvas?.evaluate(
      (element) => element === document.querySelector("canvas"),
    ),
  ).toBe(true);
  await page.screenshot({
    path: "test-results/fleet-refreshed.png",
    fullPage: true,
  });
});

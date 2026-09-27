import { expect, test } from "@playwright/test";
import { recordedApi } from "./recorded-api";

test("Observe gives the geographic field the primary desktop composition", async ({
  page,
}, testInfo) => {
  await recordedApi(page);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/fleet");
  await expect(page.locator("canvas")).toHaveCount(1);
  await expect(
    page.getByRole("combobox", { name: "Geographic measure" }),
  ).toBeVisible();
  const stage = await page.locator(".grid-stage").boundingBox();
  const quantities = await page.locator(".headline-quantities").boundingBox();
  expect(stage).not.toBeNull();
  expect(quantities).not.toBeNull();
  expect(stage!.y).toBeLessThan(280);
  expect(stage!.height).toBeGreaterThanOrEqual(520);
  expect(quantities!.y).toBeGreaterThan(stage!.y + stage!.height);
  await page.screenshot({ path: testInfo.outputPath("cinematic-observe.png") });
});

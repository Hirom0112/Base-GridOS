import { expect, test } from "@playwright/test";
import { recordedApi } from "./recorded-api";

test("context loss exposes the same geographic evidence and restores one canvas", async ({
  page,
}, testInfo) => {
  await recordedApi(page);
  await page.goto("/fleet");
  await expect(
    page.getByText("3D geographic field", { exact: true }),
  ).toBeVisible();
  const canvas = page.locator("canvas[data-living-grid]");
  await canvas.evaluate((element) => {
    const gl = (element as HTMLCanvasElement).getContext("webgl2");
    const extension = gl?.getExtension("WEBGL_lose_context");
    if (!extension) throw new Error("Context-loss extension unavailable");
    extension.loseContext();
  });
  await expect(
    page.getByText("Geographic fallback · WebGL context lost"),
  ).toBeVisible();
  await expect(
    page.getByRole("img", { name: /H3 power relief/ }),
  ).toBeVisible();
  await page.getByText("Inspect the geographic data").click();
  await expect(page.locator("tbody tr")).toHaveCount(320);
  await page.screenshot({
    path: testInfo.outputPath("context-lost.png"),
    fullPage: true,
  });
  await canvas.evaluate((element) => {
    const gl = (element as HTMLCanvasElement).getContext("webgl2");
    gl?.getExtension("WEBGL_lose_context")?.restoreContext();
  });
  await expect(
    page.getByText("3D geographic field", { exact: true }),
  ).toBeVisible();
  await expect(canvas).toBeVisible();
  await expect(canvas).toHaveCount(1);
  await page.getByRole("link", { name: "Plan a dispatch" }).click();
  await expect(
    page.getByRole("heading", { name: "Request a dispatch plan" }),
  ).toBeVisible();
});

test("a failed renderer download keeps the field and actions available", async ({
  page,
}, testInfo) => {
  await recordedApi(page);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("**/src/fleet/renderer.ts*", (route) =>
    route.abort("failed"),
  );
  await page.goto("/fleet");
  await expect(
    page.getByText("Geographic fallback · renderer unavailable"),
  ).toBeVisible();
  await expect(
    page.getByRole("img", { name: /H3 power relief/ }),
  ).toBeVisible();
  await page.getByText("Inspect the geographic data").click();
  await expect(page.locator("tbody tr")).toHaveCount(320);
  expect(errors).toEqual([]);
  await page.screenshot({
    path: testInfo.outputPath("renderer-unavailable.png"),
    fullPage: true,
  });
  await page.getByRole("link", { name: "Plan a dispatch" }).click();
  await expect(
    page.getByRole("heading", { name: "Request a dispatch plan" }),
  ).toBeVisible();
});

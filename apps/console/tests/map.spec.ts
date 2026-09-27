import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { recordedApi } from "./recorded-api";

test.beforeEach(async ({ page }) => {
  await recordedApi(page);
  await page.route("**/geo/*", async (route) => {
    const name = new URL(route.request().url()).pathname.split("/").at(-1);
    const body = await readFile(
      resolve(process.cwd(), `../../testdata/fixtures/geo/${name}`),
      "utf8",
    );
    await route.fulfill({ contentType: "application/json", body });
  });
});

for (const width of [390, 1440]) {
  test(`map preserves evidence and releases competing canvases at ${width}`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.goto("/fleet");
    await expect(page.locator("canvas[data-living-grid]")).toHaveCount(1);
    await page.getByRole("link", { name: "Explore map" }).click();
    await expect(page.getByText("Geographic map ready")).toBeVisible();
    await expect(page.locator("canvas")).toHaveCount(1);
    await expect(page.locator("canvas[data-living-grid]")).toHaveCount(0);
    await page.getByLabel("Map measure").selectOption("sites");
    await page.getByRole("button", { name: "Inspect 87489d884ffffff" }).click();
    await expect(
      page.getByRole("region", { name: "Selected map cell" }),
    ).toContainText("SIMULATED");
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    expect(
      (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze())
        .violations,
    ).toEqual([]);
    await page.screenshot({
      path: `test-results/map-${width}.png`,
      fullPage: true,
    });
    await page.getByRole("link", { name: /Observe/ }).click();
    await expect(page.locator("canvas[data-living-grid]")).toHaveCount(1);
    await expect(page.locator("canvas")).toHaveCount(1);
  });
}

test("map remains usable without WebGL and with reduced motion", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.addInitScript(() => {
    HTMLCanvasElement.prototype.getContext = () => null;
  });
  await page.goto("/map");
  await expect(
    page.getByText("Map unavailable · geographic table retained"),
  ).toBeVisible();
  await expect(
    page.getByRole("img", { name: "H3 geographic overview" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Inspect 87489d884ffffff" }).click();
  await expect(
    page.getByRole("region", { name: "Selected map cell" }),
  ).toContainText("SIMULATED");
  await page.screenshot({
    path: "test-results/map-fallback.png",
    fullPage: true,
  });
});

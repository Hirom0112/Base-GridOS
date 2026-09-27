import { expect, test } from "@playwright/test";
import { recordedApi } from "./recorded-api";

for (const width of [390, 1440]) {
  test(`fleet controls and operational labels meet the primary text minimum at ${width}`, async ({
    page,
  }, testInfo) => {
    await recordedApi(page);
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/fleet");
    await expect(
      page.getByRole("heading", { name: "Operating state" }),
    ).toBeVisible();
    for (const selector of [
      ".action-button",
      ".quantity-label",
      ".fleet-count summary",
      ".fleet-health h2",
    ]) {
      const sizes = await page
        .locator(selector)
        .evaluateAll((elements) =>
          elements.map((element) =>
            Number.parseFloat(getComputedStyle(element).fontSize),
          ),
        );
      expect(sizes.length).toBeGreaterThan(0);
      expect(Math.min(...sizes), selector).toBeGreaterThanOrEqual(14);
    }
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.locator(".living-grid").scrollIntoViewIfNeeded();
    await expect(page.locator("canvas")).toHaveCount(1);
    await page.evaluate(
      () =>
        new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        ),
    );
    await page.screenshot({
      path: testInfo.outputPath(`fleet-legibility-${width}.png`),
      fullPage: true,
    });
  });
}

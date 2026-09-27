import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { recordedApi } from "./recorded-api";

for (const width of [390, 768, 1440]) {
  for (const theme of ["dark", "light"]) {
    test(`event records remain legible at ${width} in ${theme}`, async ({
      page,
    }, testInfo) => {
      await recordedApi(page);
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/events/event_austin_wave2_live_0002");
      if (theme === "light")
        await page.getByRole("button", { name: "Use light theme" }).click();
      const records = page.getByRole("list", { name: "Server audit timeline" });
      await expect(records.getByRole("listitem")).toHaveCount(15);
      await expect(records).toContainText("Safety validated → Approved");
      await expect(
        page.getByText(/Command acceptance does not prove measured delivery/),
      ).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      expect(
        (
          await new AxeBuilder({ page })
            .withTags(["wcag2a", "wcag2aa"])
            .analyze()
        ).violations,
      ).toEqual([]);
      await page.locator(".event-panel").screenshot({
        path: testInfo.outputPath(`event-history-${theme}-${width}.png`),
      });
    });
  }
}

test("event records remain available with reduced motion and no WebGL", async ({
  page,
}, testInfo) => {
  await recordedApi(page);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.addInitScript(() => {
    HTMLCanvasElement.prototype.getContext = () => null;
  });
  await page.goto("/events/event_austin_wave2_live_0002");
  await expect(
    page
      .getByRole("list", { name: "Server audit timeline" })
      .getByRole("listitem"),
  ).toHaveCount(15);
  await expect(
    page.getByText("Geographic fallback · WebGL unavailable"),
  ).toBeVisible();
  await expect(page.locator("canvas")).toHaveCount(0);
  expect(await page.evaluate(() => document.getAnimations().length)).toBe(0);
  await page.screenshot({
    path: testInfo.outputPath("event-history-fallback.png"),
    fullPage: true,
  });
});

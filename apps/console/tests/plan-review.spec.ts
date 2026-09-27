import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { recordedApi } from "./recorded-api";

for (const width of [390, 1440]) {
  for (const theme of ["dark", "light"]) {
    test(`plan evidence remains legible without WebGL at ${width} in ${theme}`, async ({
      page,
    }, testInfo) => {
      await recordedApi(page);
      await page.setViewportSize({ width, height: 900 });
      await page.emulateMedia({ reducedMotion: "reduce" });
      await page.addInitScript(() => {
        HTMLCanvasElement.prototype.getContext = () => null;
      });
      await page.goto("/dispatch/event_austin_wave2_live_0002");
      if (theme === "light")
        await page.getByRole("button", { name: "Use light theme" }).click();
      const explanation = page.getByRole("region", {
        name: "Optimization explanation",
      });
      await expect(explanation).toContainText("51,973.637 kWh");
      await expect(
        page.getByRole("region", { name: "Constraint margins" }),
      ).toContainText("2.20326299554927");
      await page
        .getByText("Modeled objective · inspect the value and costs")
        .click();
      await expect(explanation).toContainText("-1.037");
      await expect(
        page.getByRole("table", { name: "Interval feasibility" }),
      ).toContainText("1.000");
      await page
        .getByLabel("Find reserve device")
        .fill("device_2546ed64004b08202d20");
      await expect(
        page.getByRole("region", { name: "Household reserve basis" }),
      ).toContainText("COMMUNICATIONS");
      await expect(
        page.getByRole("region", { name: "Travel Flex eligibility" }),
      ).toContainText("500 cents · Fixed daily credit");
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
      await explanation.screenshot({
        path: testInfo.outputPath(`plan-review-${theme}-${width}.png`),
      });
    });
  }
}

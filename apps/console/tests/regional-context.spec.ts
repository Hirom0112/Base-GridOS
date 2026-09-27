import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { recordedApi } from "./recorded-api";

for (const width of [390, 1440]) {
  test(`regional evidence preserves source and fits at ${width}`, async ({
    page,
  }, testInfo) => {
    await recordedApi(page);
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/fleet");
    await expect(
      page.getByRole("combobox", { name: "Demo role" }),
    ).toBeEnabled();
    const context = page.getByRole("region", { name: "Austin conditions" });
    await context
      .getByText("Inspect weather and historical outage evidence")
      .click();
    await expect(context).toContainText("3.766% historical rate");
    await expect(context).toContainText("2023-01");
    await expect(context).toContainText("CONFIRMED_PUBLIC");
    await expect(context).toContainText("97 °F");
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    expect(
      (
        await new AxeBuilder({ page })
          .include(".regional-context")
          .withTags(["wcag2a", "wcag2aa"])
          .analyze()
      ).violations,
    ).toEqual([]);
    await context.screenshot({
      path: testInfo.outputPath(`regional-${width}.png`),
    });
  });
}

import { expect, test } from "@playwright/test";

test("shell keeps its static truth and keyboard path", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/");
  await expect(page.getByRole("main")).toHaveAccessibleName("Austin fleet");
  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("link", { name: "Skip to fleet overview" }),
  ).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("main")).toBeFocused();
  await expect(page.locator("canvas")).toHaveCount(0);
  expect(errors).toEqual([]);
});

for (const theme of ["dark", "light"] as const) {
  for (const width of [390, 768, 1440]) {
    test(`shell ${theme} at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/");
      if (theme === "light")
        await page.getByRole("button", { name: "Use light theme" }).click();
      await expect(
        page.getByRole("heading", { name: "Austin fleet" }),
      ).toBeVisible();
      await expect(
        page.getByRole("complementary", { name: "Fleet evidence" }),
      ).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      await expect(page).toHaveScreenshot(`shell-${theme}-${width}.png`, {
        fullPage: true,
      });
    });
  }
}

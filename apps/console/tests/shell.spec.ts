import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { recordedApi } from "./recorded-api";

test.beforeEach(async ({ page }) => {
  await recordedApi(page);
});

test("shell keeps its static truth and keyboard path", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const response = await page.goto("/");
  expect(response?.status()).toBe(200);
  await expect(page.getByRole("main")).toHaveAccessibleName("Austin fleet");
  await expect(page.getByRole("combobox", { name: "Demo role" })).toBeEnabled();
  await page.keyboard.press("Tab");
  await page.keyboard.press("Tab");
  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("link", { name: "Skip to fleet overview" }),
  ).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("main")).toBeFocused();
  await expect(page.locator("canvas")).toHaveCount(1);
  expect(errors).toEqual([]);
});

for (const theme of ["dark", "light"] as const) {
  for (const width of [390, 768, 1440]) {
    test(`shell ${theme} at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/");
      await expect(page.locator("canvas")).toHaveCount(1);
      if (theme === "light")
        await page.getByRole("button", { name: "Use light theme" }).click();
      await expect(page.locator(".console")).toHaveAttribute(
        "data-theme",
        theme,
      );
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
      expect(
        (
          await new AxeBuilder({ page })
            .withTags(["wcag2a", "wcag2aa"])
            .analyze()
        ).violations,
      ).toEqual([]);
      await page.locator(".living-grid").scrollIntoViewIfNeeded();
      await page.evaluate(
        () =>
          new Promise<void>((resolve) =>
            requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
          ),
      );
      await expect(page).toHaveScreenshot(`shell-${theme}-${width}.png`, {
        fullPage: true,
      });
    });
  }
}

test("shell stays useful without JavaScript", async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false });
  const page = await context.newPage();
  await page.goto("http://127.0.0.1:3100/fleet");
  await expect(
    page.getByRole("heading", { name: "Austin fleet" }),
  ).toBeVisible();
  await expect(
    page.getByRole("complementary", { name: "Fleet evidence" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Use light theme" }),
  ).toBeDisabled();
  await context.close();
});

test("shell remains static under reduced motion without WebGL", async ({
  browser,
}) => {
  const context = await browser.newContext({ reducedMotion: "reduce" });
  await context.addInitScript(() => {
    HTMLCanvasElement.prototype.getContext = () => null;
  });
  const page = await context.newPage();
  await recordedApi(page);
  await page.goto("http://127.0.0.1:3100/fleet");
  expect(
    await page.evaluate(() =>
      document.createElement("canvas").getContext("webgl"),
    ),
  ).toBeNull();
  await expect(
    page.getByRole("heading", { name: "Austin fleet" }),
  ).toBeVisible();
  await expect(page.getByRole("contentinfo")).toContainText(
    "Fleet observation recorded",
  );
  await page.getByRole("button", { name: "Use light theme" }).click();
  await expect(page.locator(".console")).toHaveAttribute("data-theme", "light");
  expect(await page.evaluate(() => document.getAnimations().length)).toBe(0);
  await context.close();
});

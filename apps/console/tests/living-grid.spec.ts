import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { recordedApi } from "./recorded-api";

test.beforeEach(async ({ page }) => {
  await recordedApi(page);
});

test("geography is evidenced, keyboard accessible, and survives route navigation", async ({
  page,
}) => {
  const errors: string[] = [];
  const clerkRequests: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("request", (request) => {
    if (/clerk\.(accounts|com)/.test(request.url()))
      clerkRequests.push(request.url());
  });
  await page.goto("/fleet");
  await expect(page.getByText("320 H3 cells · LZ_AEN")).toBeVisible();
  await expect(page.locator("canvas")).toHaveCount(1);
  const canvas = await page.locator("canvas").elementHandle();
  await page.getByText("Inspect the geographic data").click();
  await expect(
    page
      .getByRole("region", { name: "Geographic measurements" })
      .locator("tbody tr"),
  ).toHaveCount(320);
  const first = page
    .getByRole("region", { name: "Geographic measurements" })
    .locator("tbody button")
    .first();
  const id = await first.innerText();
  await first.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("heading", { name: id })).toBeVisible();
  await page.getByRole("link", { name: "Plan a dispatch" }).click();
  await expect(
    page.getByRole("heading", { name: "Request a dispatch plan" }),
  ).toBeVisible();
  expect(
    await canvas?.evaluate(
      (element) => element === document.querySelector("canvas"),
    ),
  ).toBe(true);
  expect(errors).toEqual([]);
  expect(clerkRequests).toEqual([]);
});

for (const width of [390, 768, 1440]) {
  for (const theme of ["dark", "light"]) {
    test(`connected fleet ${theme} at ${width}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/fleet");
      await expect(page.locator("canvas")).toHaveCount(1);
      if (theme === "light")
        await page.getByRole("button", { name: "Use light theme" }).click();
      await expect(page.locator(".headline-quantities")).toContainText(
        "42.630",
      );
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
      await page.screenshot({
        path: testInfo.outputPath(`fleet-${theme}-${width}.png`),
        fullPage: true,
      });
      await page.getByRole("link", { name: "Plan a dispatch" }).click();
      await expect(
        page.getByRole("heading", { name: "Request a dispatch plan" }),
      ).toBeVisible();
      expect(
        (
          await new AxeBuilder({ page })
            .withTags(["wcag2a", "wcag2aa"])
            .analyze()
        ).violations,
      ).toEqual([]);
      await page.screenshot({
        path: testInfo.outputPath(`dispatch-${theme}-${width}.png`),
        fullPage: true,
      });
    });
  }
}

test("reduced motion and unavailable WebGL retain the same geographic evidence", async ({
  page,
}, testInfo) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.addInitScript(() => {
    HTMLCanvasElement.prototype.getContext = () => null;
  });
  await page.goto("/fleet");
  await expect(
    page.getByText("Geographic fallback · WebGL unavailable"),
  ).toBeVisible();
  await expect(
    page.getByRole("img", { name: /H3 power relief/ }),
  ).toBeVisible();
  await page.getByText("Inspect the geographic data").click();
  await expect(
    page
      .getByRole("region", { name: "Geographic measurements" })
      .locator("tbody tr"),
  ).toHaveCount(320);
  expect(await page.evaluate(() => document.getAnimations().length)).toBe(0);
  await page.screenshot({
    path: testInfo.outputPath("fleet-no-webgl.png"),
    fullPage: true,
  });
});

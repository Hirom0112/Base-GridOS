import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { recordedApi } from "./recorded-api";

const profiles = [
  {
    name: "desktop-dark",
    width: 1440,
    height: 1000,
    theme: "dark",
    zoom: 1,
    motion: "no-preference",
    graphics: "webgl",
  },
  {
    name: "tablet-light",
    width: 768,
    height: 900,
    theme: "light",
    zoom: 1,
    motion: "no-preference",
    graphics: "webgl",
  },
  {
    name: "mobile-dark",
    width: 390,
    height: 844,
    theme: "dark",
    zoom: 1,
    motion: "no-preference",
    graphics: "webgl",
  },
  {
    name: "mobile-light",
    width: 390,
    height: 844,
    theme: "light",
    zoom: 1,
    motion: "no-preference",
    graphics: "webgl",
  },
  {
    name: "short-screen",
    width: 1280,
    height: 600,
    theme: "dark",
    zoom: 1,
    motion: "no-preference",
    graphics: "webgl",
  },
  {
    name: "zoom-200-reflow",
    width: 1440,
    height: 1000,
    theme: "light",
    zoom: 2,
    motion: "no-preference",
    graphics: "webgl",
  },
  {
    name: "no-webgl",
    width: 1440,
    height: 1000,
    theme: "dark",
    zoom: 1,
    motion: "reduce",
    graphics: "fallback",
  },
  {
    name: "reduced-motion",
    width: 390,
    height: 844,
    theme: "dark",
    zoom: 1,
    motion: "reduce",
    graphics: "webgl",
  },
] as const;

for (const profile of profiles) {
  test(`golden composition and keyboard path ${profile.name}`, async ({
    page,
  }, testInfo) => {
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (message) => {
      if (message.type() !== "error") return;
      if (
        profile.graphics === "fallback" &&
        message.text().includes("Error creating WebGL context")
      )
        return;
      errors.push(message.text());
    });
    await recordedApi(page);
    await page.setViewportSize({
      width: profile.width / profile.zoom,
      height: profile.height / profile.zoom,
    });
    await page.emulateMedia({ reducedMotion: profile.motion });
    if (profile.graphics === "fallback")
      await page.addInitScript(() => {
        HTMLCanvasElement.prototype.getContext = () => null;
      });
    await page.goto("/fleet");
    if (profile.theme === "light")
      await page.getByRole("button", { name: "Use light theme" }).click();
    await expect(
      page.getByText(
        profile.graphics === "fallback"
          ? "Geographic fallback · WebGL unavailable"
          : "3D geographic field",
        { exact: true },
      ),
    ).toBeVisible();
    await expect(page.locator("canvas[data-living-grid]")).toHaveCount(
      profile.graphics === "fallback" ? 0 : 1,
    );
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    expect(
      (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze())
        .violations,
    ).toEqual([]);
    if (profile.motion === "reduce")
      expect(
        await page.evaluate(
          () =>
            document
              .getAnimations()
              .filter((animation) => animation.playState === "running").length,
        ),
      ).toBe(0);
    await page.locator(".living-grid").scrollIntoViewIfNeeded();
    await page.evaluate(
      () =>
        new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        ),
    );
    await page.screenshot({
      path: testInfo.outputPath("fleet.png"),
      fullPage: true,
    });
    await page.getByText("Inspect the geographic data").focus();
    await page.keyboard.press("Enter");
    const measurements = page.getByRole("region", {
      name: "Geographic measurements",
    });
    await expect(measurements.locator("tbody tr")).toHaveCount(320);
    const cell = measurements.getByRole("button").first();
    const id = await cell.innerText();
    await cell.focus();
    await page.keyboard.press("Enter");
    await expect(
      page.getByRole("heading", { name: id, exact: true }),
    ).toBeVisible();
    await page.getByRole("link", { name: "Plan a dispatch" }).focus();
    await page.keyboard.press("Enter");
    await expect(
      page.getByRole("heading", { name: "Request a dispatch plan" }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    expect(
      (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze())
        .violations,
    ).toEqual([]);
    expect(errors).toEqual([]);
    await page.locator(".living-grid").scrollIntoViewIfNeeded();
    await page.evaluate(
      () =>
        new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        ),
    );
    await page.screenshot({
      path: testInfo.outputPath("planning.png"),
      fullPage: true,
    });
  });
}

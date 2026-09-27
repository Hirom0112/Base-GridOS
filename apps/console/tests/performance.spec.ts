import { readFile, writeFile } from "node:fs/promises";
import { gzipSync } from "node:zlib";
import { basename } from "node:path";
import { expect, test } from "@playwright/test";
import { recordedApi } from "./recorded-api";

type FrameMetrics = {
  cls: number;
  shifts: { value: number; at: number; nodes: string[] }[];
  longTasks: number[];
  frames: number[];
  draws: number;
  peakDraws: number;
  totalDraws: number;
  last: number;
  observing: boolean;
};
type MeasuredWindow = Window & { gridosPerformance: FrameMetrics };

function measureFrames() {
  const metrics: FrameMetrics = {
    cls: 0,
    shifts: [],
    longTasks: [],
    frames: [],
    draws: 0,
    peakDraws: 0,
    totalDraws: 0,
    last: 0,
    observing: false,
  };
  (window as MeasuredWindow).gridosPerformance = metrics;
  new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
      const shift = entry as PerformanceEntry & {
        hadRecentInput: boolean;
        value: number;
        sources: {
          node?: Element;
          previousRect: DOMRectReadOnly;
          currentRect: DOMRectReadOnly;
        }[];
      };
      if (!shift.hadRecentInput) {
        metrics.cls += shift.value;
        metrics.shifts.push({
          value: shift.value,
          at: shift.startTime,
          nodes: shift.sources.map(({ node, previousRect, currentRect }) =>
            JSON.stringify({
              html: node?.outerHTML.slice(0, 150),
              previousRect,
              currentRect,
            }),
          ),
        });
      }
    }
  }).observe({ type: "layout-shift", buffered: true });
  new PerformanceObserver((list) => {
    if (metrics.observing)
      metrics.longTasks.push(
        ...list.getEntries().map((entry) => entry.duration),
      );
  }).observe({ type: "longtask" });
  const gl = WebGL2RenderingContext.prototype;
  const elements = gl.drawElements;
  gl.drawElements = function (...args) {
    metrics.draws++;
    metrics.totalDraws++;
    return elements.apply(this, args);
  };
  const arrays = gl.drawArrays;
  gl.drawArrays = function (...args) {
    metrics.draws++;
    metrics.totalDraws++;
    return arrays.apply(this, args);
  };
  const instanced = gl.drawElementsInstanced;
  gl.drawElementsInstanced = function (...args) {
    metrics.draws++;
    metrics.totalDraws++;
    return instanced.apply(this, args);
  };
  function frame(at: number) {
    if (metrics.observing && metrics.last)
      metrics.frames.push(at - metrics.last);
    metrics.peakDraws = Math.max(metrics.peakDraws, metrics.draws);
    metrics.draws = 0;
    metrics.last = at;
    requestAnimationFrame(frame);
  }
  requestAnimationFrame(frame);
}

for (const width of [390, 1440]) {
  test(`production fleet budgets at ${width}`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    await recordedApi(page);
    await page.addInitScript(measureFrames);
    await page.goto("/fleet");
    await expect(
      page.getByText("3D geographic field", { exact: true }),
    ).toBeVisible();
    await expect(page.locator("canvas[data-living-grid]")).toBeVisible();
    await page.evaluate(() => {
      const metrics = (window as MeasuredWindow).gridosPerformance;
      metrics.observing = true;
      metrics.last = 0;
      metrics.peakDraws = 0;
    });
    for (let index = 0; index < 6; index++) {
      await page
        .getByRole("button", {
          name: index % 2 === 0 ? "Use light theme" : "Use dark theme",
        })
        .click();
      await page
        .getByRole("combobox", { name: /Geographic measure/ })
        .selectOption(index % 2 === 0 ? "sent" : "installed");
      await page.evaluate(
        () =>
          new Promise<void>((resolve) => {
            let count = 0;
            function frame() {
              if (++count === 12) resolve();
              else requestAnimationFrame(frame);
            }
            requestAnimationFrame(frame);
          }),
      );
    }
    const metrics = await page.evaluate(() => {
      const metrics = (window as MeasuredWindow).gridosPerformance;
      metrics.observing = false;
      return metrics;
    });
    const urls = await page.evaluate(() =>
      performance
        .getEntriesByType("resource")
        .map((entry) => entry.name)
        .filter((name) => new URL(name).pathname.endsWith(".js")),
    );
    const files = [
      ...new Set(urls.map((url) => basename(new URL(url).pathname))),
    ];
    const payload = await Promise.all(
      files.map(async (file) => ({
        file,
        gzipBytes: gzipSync(await readFile(`dist/client/assets/${file}`))
          .length,
      })),
    );
    const spatial = payload.filter(({ file }) =>
      /^(renderer|h3|living-grid)-/.test(file),
    );
    const critical = payload.filter(
      ({ file }) => !spatial.some((entry) => entry.file === file),
    );
    const criticalBytes = critical.reduce(
      (total, entry) => total + entry.gzipBytes,
      0,
    );
    const spatialBytes = spatial.reduce(
      (total, entry) => total + entry.gzipBytes,
      0,
    );
    const fps =
      1000 /
      (metrics.frames.reduce((total, frame) => total + frame, 0) /
        metrics.frames.length);
    const report = {
      viewport: { width, height: 1000 },
      browser: await page.evaluate(() => navigator.userAgent),
      fixture: "recorded 320 H3 cells",
      profile:
        "production local-identity build, headless Chromium, no CPU throttle, theme and measure interaction",
      criticalBytes,
      spatialBytes,
      fps,
      cls: metrics.cls,
      shifts: metrics.shifts,
      peakDraws: metrics.peakDraws,
      totalDraws: metrics.totalDraws,
      interactionLongTasks: metrics.longTasks,
      assets: payload,
    };
    await writeFile(
      testInfo.outputPath("performance.json"),
      JSON.stringify(report, null, 2),
    );
    console.log(JSON.stringify(report));
    expect(metrics.totalDraws).toBeGreaterThan(0);
    expect(criticalBytes).toBeLessThan(250000);
    expect(spatialBytes).toBeLessThan(3000000);
    expect(metrics.peakDraws).toBeLessThan(width < 768 ? 60 : 100);
    expect(fps).toBeGreaterThanOrEqual(width < 768 ? 30 : 55);
    expect(metrics.cls).toBeLessThan(0.1);
    expect(
      metrics.longTasks.filter((duration) => duration > 50).length,
    ).toBeLessThan(2);
    await page.screenshot({
      path: testInfo.outputPath(`production-${width}.png`),
      fullPage: true,
    });
  });
}

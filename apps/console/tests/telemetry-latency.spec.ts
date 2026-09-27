import { createServer } from "node:http";
import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { recordedApi } from "./recorded-api";

function observation(at: string, deliveredMw: number, deliveredState: string) {
  const metadata = {
    timestamp: at,
    freshness: "0s",
    provenanceMix: [
      { provenance: "DATA_PROVENANCE_SIMULATED", recordCount: "10" },
    ],
  };
  return {
    event: {
      eventId: "event_austin_wave2_live_0002",
      state: "DISPATCH_EVENT_STATE_SENT",
    },
    observedAt: at,
    fleet: {
      sentMw: 1,
      acknowledgedMw: 0.8,
      deliveredMw,
      deliveredState,
      metadata,
    },
    h3: [
      {
        h3Cell: "874898431ffffff",
        power: {
          sentMw: 0.1,
          acknowledgedMw: 0.08,
          deliveredMw: deliveredMw / 10,
          deliveredState,
          metadata,
        },
        metadata,
      },
    ],
  };
}

function envelope(value: ReturnType<typeof observation>) {
  const body = Buffer.from(JSON.stringify(value));
  const header = Buffer.alloc(5);
  header.writeUInt32BE(body.length, 1);
  return Buffer.concat([header, body]);
}

for (const { width, mode } of [
  { width: 390, mode: "dark" },
  { width: 1440, mode: "dark" },
  { width: 390, mode: "light" },
  { width: 1440, mode: "fallback" },
]) {
  test(`streamed telemetry remains distinct and visible within five seconds at ${width} ${mode}`, async ({
    page,
  }, testInfo) => {
    let emittedAt = 0;
    let closed = 0;
    const server = createServer((request, response) => {
      response.setHeader("Access-Control-Allow-Origin", "*");
      response.setHeader("Access-Control-Allow-Headers", "*");
      response.setHeader("Access-Control-Allow-Methods", "POST, OPTIONS");
      if (request.method === "OPTIONS") {
        response.end();
        return;
      }
      response.setHeader("Content-Type", "application/connect+json");
      response.write(
        envelope(observation("2026-09-26T12:00:00Z", 0, "VALUE_STATE_UNKNOWN")),
      );
      const timer = setTimeout(() => {
        emittedAt = Date.now();
        response.write(
          envelope(
            observation("2026-09-26T12:00:01Z", 0.65, "VALUE_STATE_PRESENT"),
          ),
        );
      }, 1500);
      response.on("close", () => {
        closed += 1;
        clearTimeout(timer);
      });
    });
    await new Promise<void>((resolve) =>
      server.listen(0, "127.0.0.1", resolve),
    );
    try {
      const address = server.address();
      if (!address || typeof address === "string")
        throw new Error("Stream server address missing");
      await recordedApi(page);
      await page.addInitScript((url) => {
        const nativeFetch = window.fetch;
        window.fetch = (input, options) => {
          const target = input instanceof Request ? input.url : String(input);
          if (target.endsWith("/gridos.v1.EventsService/WatchEvent"))
            return nativeFetch(url, options);
          return nativeFetch(input, options);
        };
      }, `http://127.0.0.1:${address.port}/`);
      await page.setViewportSize({ width, height: 1000 });
      if (mode === "fallback") {
        await page.emulateMedia({ reducedMotion: "reduce" });
        await page.addInitScript(() => {
          HTMLCanvasElement.prototype.getContext = () => null;
        });
      }
      await page.goto("/events/event_austin_wave2_live_0002");
      await expect(page.getByText("Delivery unknown")).toBeVisible();
      if (mode === "light")
        await page.getByRole("button", { name: "Use light theme" }).click();
      await expect(page.getByText("0.650 MW")).toBeVisible({ timeout: 5000 });
      expect(Date.now() - emittedAt).toBeLessThan(5000);
      await expect(page.getByText("Stream connected")).toBeVisible();
      await expect(page.getByText("1.000 MW", { exact: true })).toBeVisible();
      await expect(page.getByText("0.800 MW", { exact: true })).toBeVisible();
      expect(
        (
          await new AxeBuilder({ page })
            .withTags(["wcag2a", "wcag2aa"])
            .analyze()
        ).violations,
      ).toEqual([]);
      await page.locator(".live-response").screenshot({
        path: testInfo.outputPath(`live-response-${width}-${mode}.png`),
      });
      await page
        .getByRole("combobox", { name: /Geographic measure/ })
        .selectOption("delivered");
      await page.getByText("Inspect the geographic data").click();
      const row = page.getByRole("row").filter({ hasText: "874898431ffffff" });
      await expect(row).toContainText("0.065");
      await expect(page.locator(".geography-table")).toContainText(
        "Unavailable",
      );
      await page.getByText("Inspect the geographic data").click();
      await page.locator(".living-grid").scrollIntoViewIfNeeded();
      await page.locator(".living-grid").screenshot({
        path: testInfo.outputPath(`live-geography-${width}-${mode}.png`),
      });
      await page
        .getByRole("combobox", { name: "Demo role" })
        .selectOption("member");
      await expect.poll(() => closed).toBeGreaterThan(0);
    } finally {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    }
  });
}

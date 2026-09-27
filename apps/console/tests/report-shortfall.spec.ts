import { readFile } from "node:fs/promises";
import { fromJsonString, toJsonString } from "@bufbuild/protobuf";
import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { z } from "zod";
import { GetEventResponseSchema } from "../src/api/gen/gridos/v1/api_pb";
import { recordedApi } from "./recorded-api";

for (const width of [390, 1440]) {
  test(`shortfall remains legible without WebGL at ${width}`, async ({
    page,
  }, testInfo) => {
    await recordedApi(page);
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.addInitScript(() => {
      HTMLCanvasElement.prototype.getContext = () => null;
    });
    const event = fromJsonString(
      GetEventResponseSchema,
      await readFile(
        "../../testdata/fixtures/api/DispatchService/GetEvent.json",
        "utf8",
      ),
    );
    if (!event.event) throw new Error("Event fixture required");
    event.event.eventId = "event-report-a";
    await page.route("**/gridos.v1.DispatchService/GetEvent", async (route) => {
      await route.fulfill({
        contentType: "application/json",
        body: toJsonString(GetEventResponseSchema, event),
      });
    });
    const envelope = z
      .object({ reportJson: z.string() })
      .parse(
        JSON.parse(
          await readFile(
            "../../testdata/fixtures/api/ReportService/GetEventReport.json",
            "utf8",
          ),
        ),
      );
    const recorded = z
      .record(z.string(), z.unknown())
      .parse(JSON.parse(envelope.reportJson));
    const interval = {
      begin: "2026-09-27T12:00:00Z",
      end: "2026-09-27T12:15:00Z",
    };
    await page.route(
      "**/gridos.v1.ReportService/GetEventReport",
      async (route) => {
        await route.fulfill({
          json: {
            reportJson: JSON.stringify({
              ...recorded,
              planned_shortfall: [
                {
                  ...interval,
                  requested_kw: 1000,
                  feasible_kw: 800,
                  shortfall_kw: 200,
                  reasons: ["RESERVE"],
                },
              ],
              delivery_shortfall: [
                {
                  ...interval,
                  requested_kwh: 250,
                  measured_delivered_kwh: 125,
                  shortfall_kwh: 125,
                  coverage: 0.5,
                  value_kind: "MEASURED",
                },
                {
                  begin: interval.end,
                  end: "2026-09-27T12:30:00Z",
                  requested_kwh: 250,
                  coverage: 0,
                  value_kind: "UNKNOWN",
                },
              ],
            }),
          },
        });
      },
    );
    await page.goto("/events/event-report-a/report");
    if (width === 1440)
      await page.getByRole("button", { name: "Use light theme" }).click();
    const report = page.getByRole("region", {
      name: "Event report",
      exact: true,
    });
    await expect(report).toContainText("SIMULATED");
    await expect(
      report.getByRole("table", { name: "Planned shortfall" }),
    ).toContainText("200.000 kW");
    const delivery = report.getByRole("table", { name: "Delivery shortfall" });
    await expect(delivery).toContainText("50.0%");
    await expect(delivery.getByText("Unavailable")).toHaveCount(2);
    await expect(delivery).toContainText("UNKNOWN");
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    expect(
      (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze())
        .violations,
    ).toEqual([]);
    await report
      .getByRole("region", { name: "Delivery shortfall table" })
      .focus();
    await expect(
      report.getByRole("region", { name: "Delivery shortfall table" }),
    ).toBeFocused();
    await report.screenshot({
      path: testInfo.outputPath(`shortfall-${width}.png`),
    });
  });
}

import { create, toJsonString } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { ListEventCommandsResponseSchema } from "../src/api/gen/gridos/v1/events_pb";
import { openEvidence, recordedApi } from "./recorded-api";

for (const width of [390, 1440]) {
  test(`command receipts and measured intervals stay distinct at ${width}`, async ({
    page,
  }, testInfo) => {
    await recordedApi(page);
    const at = timestampFromDate(new Date("2026-09-27T12:00:00Z"));
    const end = timestampFromDate(new Date("2026-09-27T12:05:00Z"));
    const response = create(ListEventCommandsResponseSchema, {
      commands: [4, 0].map((setpointKw, index) => ({
        intent: {
          commandId: `command-${index}`,
          eventId: "event_austin_wave2_live_0002",
          deviceId: "device-1",
          planVersion: 1n,
          generation: BigInt(index),
          setpointKw,
          issuedAt: at,
          effectiveAt: at,
          expiresAt: end,
        },
        lifecycleState: 3,
        stateRecordedAt: at,
        receipt: {
          commandId: `command-${index}`,
          acknowledgementId: `receipt-${index}`,
          receiptStatus: 1,
          gatewayId: "gateway-1",
          receivedAt: at,
        },
      })),
      verificationIntervals: [
        {
          beginTime: at,
          endTime: end,
          requestedKw: 4,
          commandedKw: 4,
          deliveredKw: 3.5,
          trackingErrorKw: -0.5,
          confidence: 0.9,
          measurementBoundary: "METER_NET_EXPORT",
          baselineMethod: "DIRECT",
          valueKind: "MEASURED",
        },
      ],
    });
    await page.route("**/gridos.v1.EventsService/ListEventCommands", (route) =>
      route.fulfill({
        contentType: "application/json",
        body: toJsonString(ListEventCommandsResponseSchema, response),
      }),
    );
    await page.route("**/gridos.v1.ReportService/GetEventReport", (route) =>
      route.fulfill({
        json: {
          reportJson: JSON.stringify({
            EventID: "event_austin_wave2_live_0002",
            ReserveCompliance: {
              DevicesExpected: 100,
              DevicesObserved: 99,
              MinimumMarginKWh: -0.25,
              DevicesTouchedFloor: 1,
              ObservationGaps: 1,
              ValueKind: "MEASURED",
              Provenance: ["SIMULATED", "FROZEN_EFFECTIVE_RESERVE"],
            },
          }),
        },
      }),
    );
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/events/event_austin_wave2_live_0002");
    await openEvidence(page);
    const commands = page.getByRole("region", { name: "Command fan-out" });
    await expect(commands).toContainText("ACKNOWLEDGED");
    await expect(commands).toContainText("2026-09-27T12:05:00.000Z");
    await expect(
      page.getByRole("region", { name: "Safe return evidence" }),
    ).toContainText("1 zero-setpoint intents · 1 accepted receipts");
    await expect(
      page.getByRole("region", { name: "Measured interval verification" }),
    ).toContainText("3.5 kW · MEASURED");
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    expect(
      (
        await new AxeBuilder({ page })
          .include(".event-report")
          .withTags(["wcag2a", "wcag2aa"])
          .analyze()
      ).violations,
    ).toEqual([]);
    await expect(
      page.getByRole("region", { name: "Reserve protection evidence" }),
    ).toContainText("-0.250 kWh");
    await commands
      .locator("..")
      .screenshot({ path: testInfo.outputPath(`commands-${width}.png`) });
  });
}

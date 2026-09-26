import { expect, test } from "@playwright/test";
import { fromJson, toJsonString, create } from "@bufbuild/protobuf";
import AxeBuilder from "@axe-core/playwright";
import {
  EmergencyStopRequestSchema,
  EmergencyStopResponseSchema,
} from "../src/api/gen/gridos/v1/api_pb";
import { recordedApi } from "./recorded-api";

for (const width of [390, 1440]) {
  test(`emergency stop is explicit and request-only at ${width}`, async ({
    page,
  }) => {
    await recordedApi(page);
    await page.setViewportSize({ width, height: 1000 });
    let calls = 0;
    await page.route(
      "**/rpc/gridos.v1.EventsService/EmergencyStop",
      async (route) => {
        const request = fromJson(
          EmergencyStopRequestSchema,
          route.request().postDataJSON(),
        );
        expect(request.reason).toBe("Unexpected response");
        expect(request.requestedBy).toBe("local-operator");
        expect(request.idempotencyKey).not.toBe("");
        calls += 1;
        await route.fulfill({
          contentType: "application/json",
          body: toJsonString(
            EmergencyStopResponseSchema,
            create(EmergencyStopResponseSchema, {
              stopRequested: true,
              emergencyStop: {
                ...request,
                $typeName: "gridos.v1.EmergencyStop",
                emergencyStopId: "stop-receipt-1",
              },
            }),
          ),
        });
      },
    );
    await page.goto("/events/event_austin_wave1_0004");
    await page.getByRole("button", { name: "Request emergency stop" }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.getByLabel("Reason for stopping").fill("Unexpected response");
    await page.getByLabel("Type STOP to confirm").fill("STOP");
    expect(calls).toBe(0);
    expect(
      (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze())
        .violations,
    ).toEqual([]);
    await page.screenshot({ path: `test-results/stop-confirm-${width}.png` });
    await page.getByRole("button", { name: "Confirm stop request" }).click();
    await expect(
      page.getByRole("region", { name: "Emergency stop" }).getByRole("status"),
    ).toContainText("STOP REQUESTED");
    await expect(page.getByText(/not confirmed stopped/)).toBeVisible();
    expect(calls).toBe(1);
    await page
      .locator(".stop-control")
      .screenshot({ path: `test-results/stop-requested-${width}.png` });
  });
}

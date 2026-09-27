import { expect, test } from "@playwright/test";
import { recordedApi } from "./recorded-api";

for (const width of [390, 1440]) {
  test(`approval blocks denied authorization and sends the signed assertion at ${width}`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.addInitScript(() => {
      HTMLCanvasElement.prototype.getContext = () => null;
    });
    await recordedApi(page);
    let authorize = false;
    const commands: { assertion: string | undefined; key: string }[] = [];
    await page.route("**/local/step-up", async (route) => {
      expect(route.request().postDataJSON()).toEqual({
        action: "APPROVE_EVENT",
        event_id: "event_austin_wave2_live_0002",
        plan_version: 1,
      });
      expect(route.request().headers()["x-gridos-role"]).toBe("approver");
      await route.fulfill({
        status: authorize ? 200 : 403,
        contentType: "application/json",
        body: JSON.stringify(
          authorize ? { assertion: "browser-test-assertion" } : {},
        ),
      });
    });
    await page.route(
      "**/gridos.v1.DispatchService/ApproveEvent",
      async (route) => {
        commands.push({
          assertion: route.request().headers()["x-gridos-step-up"],
          key: String(route.request().postDataJSON().idempotencyKey),
        });
        await route.fulfill({ contentType: "application/json", body: "{}" });
      },
    );
    await page.goto("/dispatch/event_austin_wave2_live_0002");
    await page.getByLabel("Demo role").selectOption("approver");
    await page.getByRole("button", { name: "Review approval" }).click();
    await page.getByLabel("Type plan version 1").fill("1");
    await page.getByRole("button", { name: "Confirm approval" }).click();
    await expect(page.getByRole("alert")).toContainText(
      "Step-up authorization denied",
    );
    expect(commands).toEqual([]);
    await page.getByRole("dialog").screenshot({
      path: testInfo.outputPath(`approval-denied-${width}.png`),
    });
    authorize = true;
    await page.getByRole("button", { name: "Confirm approval" }).click();
    await expect(page.getByRole("dialog")).not.toBeVisible();
    expect(commands).toHaveLength(1);
    expect(commands[0]?.assertion).toBe("browser-test-assertion");
    expect(commands[0]?.key).toMatch(/^[0-9a-f-]{36}$/);
  });
}

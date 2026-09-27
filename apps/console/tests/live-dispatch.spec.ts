import { expect, test } from "@playwright/test";
import { z } from "zod";

const assertionSchema = z.object({ assertion: z.string().min(1) });

test("live demo receives event stream evidence without launching commands", async ({
  page,
}, testInfo) => {
  await page.goto("/dispatch/new");
  await page
    .getByLabel("Start time")
    .fill(new Date(Date.now() + 120000).toISOString().slice(0, 16));
  await page
    .getByLabel("End time")
    .fill(new Date(Date.now() + 600000).toISOString().slice(0, 16));
  await page.getByLabel("Target power (MW)").fill("0.001");
  await page.getByRole("button", { name: "Create dispatch plan" }).click();
  await page.getByRole("link", { name: "Execution", exact: true }).click();
  await expect(page.getByText("Stream connected")).toBeVisible({
    timeout: 15000,
  });
  await expect(
    page.getByRole("region", { name: "Measured event response" }),
  ).toContainText("SIMULATED");
  await expect(
    page.getByRole("region", { name: "Measured event response" }),
  ).toContainText("Delivery unknown");
  await page
    .locator(".live-response")
    .screenshot({ path: testInfo.outputPath("live-stream-actual.png") });
  console.log(`LIVE STREAM ${page.url()}`);
});

test("live demo launches and stops a simulated event with enforced step-up", async ({
  page,
  request,
}, testInfo) => {
  const now = new Date().toISOString();
  const receipt = await request.post(
    "http://127.0.0.1:28080/gridos.v1.TelemetryService/PublishTelemetry",
    {
      headers: { Authorization: "Bearer local-gateway" },
      data: {
        gatewayId: "demo-gateway",
        observations: [
          {
            observationId: crypto.randomUUID(),
            deviceId: "device_bf95b2c3b700fe5bbac6",
            sequence: String(Date.now() * 1000),
            observationTime: now,
            valueState: "VALUE_STATE_PRESENT",
            stateOfEnergyPercent: 74,
            onGrid: {
              observedAt: now,
              estimatedBackupHoursAtCurrentUsage: 4,
              estimatedBackupHoursAt750Watts: 12,
            },
          },
        ],
      },
    },
  );
  expect(receipt.ok(), await receipt.text()).toBe(true);
  await page.goto("/dispatch/new");
  await page
    .getByLabel("Start time")
    .fill(new Date(Date.now() + 120000).toISOString().slice(0, 16));
  await page
    .getByLabel("End time")
    .fill(new Date(Date.now() + 600000).toISOString().slice(0, 16));
  await page.getByLabel("Target power (MW)").fill("0.001");
  await page.getByRole("button", { name: "Create dispatch plan" }).click();
  await expect(
    page.getByRole("heading", { name: "Safety validated" }),
  ).toBeVisible();
  await page.getByLabel("Demo role").selectOption("approver");
  await page.getByRole("button", { name: "Review approval" }).click();
  await page.getByLabel("Type plan version 1").fill("1");
  await page.screenshot({ path: testInfo.outputPath("live-approval.png") });
  await page.getByRole("button", { name: "Confirm approval" }).click();
  await expect(
    page.getByRole("heading", { name: "Approved", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Review launch" }).click();
  await page.getByLabel("Type plan version 1").fill("1");
  await page.getByRole("button", { name: "Confirm launch" }).click();
  await expect(
    page.getByRole("heading", { name: /Acknowledged or uncertain|Sent/ }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Execution", exact: true }).click();
  await expect(
    page.getByText("Command state alone does not establish measured delivery"),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("live-execution.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "Request emergency stop" }).click();
  await page
    .getByLabel("Reason for stopping")
    .fill("Live console verification complete");
  await page.getByLabel("Type STOP to confirm").fill("STOP");
  const issued = page.waitForResponse((response) =>
    response.url().endsWith("/local/step-up"),
  );
  const stopped = page.waitForRequest((request) =>
    request.url().endsWith("/EmergencyStop"),
  );
  const stopResponse = page.waitForResponse((response) =>
    response.url().endsWith("/EmergencyStop"),
  );
  await page.getByRole("button", { name: "Confirm stop request" }).click();
  const assertionResponse = await issued;
  expect(assertionResponse.status()).toBe(200);
  const assertion = assertionSchema.parse(
    await assertionResponse.json(),
  ).assertion;
  expect((await stopped).headers()["x-gridos-step-up"]).toBe(assertion);
  const stopReceipt = await stopResponse;
  expect(stopReceipt.ok(), await stopReceipt.text()).toBe(true);
  await expect(
    page.getByRole("region", { name: "Emergency stop" }),
  ).toContainText("STOP REQUESTED");
  await expect(
    page.getByRole("region", { name: "Emergency stop" }),
  ).toContainText("not confirmed stopped");
  await page.getByRole("link", { name: "Report", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Event evidence" }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("live-report.png"),
    fullPage: true,
  });
  console.log(`LIVE EVENT ${page.url()}`);
});

test("live approval passes the server-issued step-up assertion without launching", async ({
  page,
}) => {
  await page.goto("/dispatch/new");
  await page
    .getByLabel("Start time")
    .fill(new Date(Date.now() + 120000).toISOString().slice(0, 16));
  await page
    .getByLabel("End time")
    .fill(new Date(Date.now() + 600000).toISOString().slice(0, 16));
  await page.getByLabel("Target power (MW)").fill("0.001");
  await page.getByRole("button", { name: "Create dispatch plan" }).click();
  await expect(
    page.getByRole("heading", { name: "Safety validated" }),
  ).toBeVisible({ timeout: 20000 });
  await page.getByLabel("Demo role").selectOption("approver");
  await page.getByRole("button", { name: "Review approval" }).click();
  await page.getByLabel("Type plan version 1").fill("1");
  const issued = page.waitForResponse((response) =>
    response.url().endsWith("/local/step-up"),
  );
  const approvalResponse = page.waitForResponse((response) =>
    response.url().endsWith("/ApproveEvent"),
  );
  const approved = page.waitForRequest((request) =>
    request.url().endsWith("/ApproveEvent"),
  );
  await page.getByRole("button", { name: "Confirm approval" }).click();
  const assertionResponse = await issued;
  expect(assertionResponse.status()).toBe(200);
  const assertion = assertionSchema.parse(
    await assertionResponse.json(),
  ).assertion;
  const approvalRequest = await approved;
  expect(approvalRequest.headers()["x-gridos-step-up"]).toBe(assertion);
  const receipt = await approvalResponse;
  expect(receipt.ok(), await receipt.text()).toBe(true);
  await expect(
    page.getByRole("heading", { name: "Approved", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("No launch record returned")).toBeVisible();
  console.log(
    `LIVE STEP-UP: signer 200; matching assertion forwarded; APPROVED; no launch; ${new URL(page.url()).pathname}`,
  );
});

test("live forecasts, unsafe validation, report and replay preserve planning evidence", async ({
  page,
}, testInfo) => {
  await page.goto("/dispatch/new");
  await page
    .getByLabel("Start time")
    .fill(new Date(Date.now() + 120000).toISOString().slice(0, 16));
  await page
    .getByLabel("End time")
    .fill(new Date(Date.now() + 600000).toISOString().slice(0, 16));
  await page.getByLabel("Target power (MW)").fill("0.001");
  await page.getByRole("button", { name: "Create dispatch plan" }).click();
  await expect(
    page.getByRole("heading", { name: "Safety validated", exact: true }),
  ).toBeVisible({ timeout: 45000 });
  const forecasts = page.getByRole("region", { name: "Forecast intervals" });
  await expect(forecasts).toContainText("MODELED");
  await expect(forecasts.getByRole("table")).toContainText("kWh");
  await expect(forecasts.getByRole("table")).toContainText("DERIVED");
  await expect(
    page.getByRole("region", { name: "Solver fallback" }),
  ).toContainText(/No fallback recorded|Fallback used/);
  await page
    .getByRole("button", { name: "Validate unsafe alternative" })
    .click();
  const validation = page.getByRole("region", {
    name: "Unsafe alternative validation",
  });
  await expect(validation).toContainText("Alternative rejected");
  await expect(validation.getByRole("list")).toContainText(
    /POWER|RESERVE|ENERGY/,
  );
  await expect(
    page.getByRole("heading", { name: "Safety validated", exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Report", exact: true }).click();
  const report = page.getByRole("region", {
    name: "Event report",
    exact: true,
  });
  await expect(report).toContainText("SIMULATED");
  await expect(report).toContainText("Data gaps");
  await page.getByRole("button", { name: "Replay event", exact: true }).click();
  await expect(
    page.getByRole("region", { name: "Event replay", exact: true }),
  ).toContainText("IDENTICAL", { timeout: 30000 });
  await expect(
    page.getByRole("slider", { name: "Replay position" }),
  ).toBeVisible();
  await page
    .locator(".event-panel")
    .screenshot({ path: testInfo.outputPath("live-report-replay.png") });
  console.log(`LIVE REPORT REPLAY ${page.url()}`);
});

test("live Austin context retains regional geography and source dates", async ({
  page,
}) => {
  await page.goto("/fleet");
  await expect(page.getByLabel("Demo role")).toBeEnabled();
  const response = page.waitForRequest((request) =>
    request.url().endsWith("/GetMarketContext"),
  );
  await page.reload();
  expect((await response).postDataJSON()).toEqual({
    settlementPoint: "LZ_AEN",
    weatherZone: "SOUTH_C",
  });
  await expect(page.getByLabel("Demo role")).toBeEnabled();
  await page.getByText("Inspect Austin markets · LZ_AEN and SOUTH_C").click();
  const prices = page.getByRole("table", {
    name: "Day-ahead reference prices",
  });
  await expect(prices).toContainText("LZ_AEN");
  await expect(prices).toContainText("CONFIRMED_PUBLIC");
  await expect(prices).toContainText(/2025-01-/);
  await expect(
    page.getByRole("table", { name: "Reference system load" }),
  ).toContainText("SOUTH_C");
});

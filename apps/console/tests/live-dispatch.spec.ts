import { expect, test } from "@playwright/test";
import { z } from "zod";

const assertionSchema = z.object({ assertion: z.string().min(1) });

test("live demo receives event stream evidence without launching commands", async ({
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
    .screenshot({ path: "test-results/live-stream-actual.png" });
  console.log(`LIVE STREAM ${page.url()}`);
});

test("live demo creates, validates, approves, and launches a simulated event", async ({
  page,
  request,
}) => {
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
  await page.screenshot({ path: "test-results/live-approval.png" });
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
    page.getByText("Awaiting measured delivery evidence"),
  ).toBeVisible();
  await page.screenshot({
    path: "test-results/live-execution.png",
    fullPage: true,
  });
  await page.getByRole("link", { name: "Report", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Event evidence" }),
  ).toBeVisible();
  await page.screenshot({
    path: "test-results/live-report.png",
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

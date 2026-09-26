import { expect, test } from "@playwright/test";

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

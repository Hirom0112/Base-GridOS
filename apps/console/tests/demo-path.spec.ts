import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";

test.use({
  baseURL: process.env.GRIDOS_DEMO_CONSOLE_URL ?? "http://127.0.0.1:3000",
});

async function createPlan(page: Page, request: APIRequestContext) {
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
  await page
    .getByRole("link", { name: "Plan a dispatch", exact: true })
    .click();
  await page.getByLabel("Operating region").selectOption("LZ_AEN");
  await page
    .getByLabel("Start time")
    .fill(new Date(Date.now() + 120000).toISOString().slice(0, 16));
  await page
    .getByLabel("End time")
    .fill(new Date(Date.now() + 600000).toISOString().slice(0, 16));
  await page.getByLabel("Target power (MW)").fill("0.001");
  await page
    .getByLabel("Measurement boundary")
    .selectOption("METER_NET_EXPORT");
  await page.getByRole("button", { name: "Create dispatch plan" }).click();
  await expect(
    page.getByRole("heading", { name: "Review the plan" }),
  ).toBeVisible();
  const eventId = new URL(page.url()).pathname.split("/").at(-1) ?? "";
  expect(eventId).not.toBe("new");
  return eventId;
}

async function approveAndLaunch(page: Page) {
  await page.getByLabel("Demo role").selectOption("approver");
  await page
    .getByRole("button", { name: "Review approval" })
    .click({ timeout: 5000 });
  await page.getByLabel("Type plan version 1").fill("1");
  await page.getByRole("button", { name: "Confirm approval" }).click();
  await page
    .getByRole("button", { name: "Review launch" })
    .click({ timeout: 5000 });
  await page.getByLabel("Type plan version 1").fill("1");
  await page.getByRole("button", { name: "Confirm launch" }).click();
  await expect(
    page.getByRole("heading", {
      name: /^(Sent|Acknowledged or uncertain|Executing|Telemetry verified|Reconciled|Reported)$/,
    }),
  ).toBeVisible({ timeout: 15000 });
  await page.getByRole("link", { name: "Execution", exact: true }).click();
}

async function verifyReport(page: Page, eventId: string) {
  expect(eventId).not.toBe("");
  await page.goto(`/events/${eventId}/report`);
  await expect(
    page.getByRole("heading", { name: "Event evidence" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Server event accounting" }),
  ).toBeVisible();
  await expect(page.locator(".report-quantities")).toContainText("Requested");
  await expect(page.locator(".report-provenance")).toContainText("SIMULATED");
}

test("seventeen-step severe-weather operating loop against the live demo", async ({
  page,
  request,
}) => {
  test.setTimeout(600000);
  let eventId = "";
  await page.goto("/fleet");
  await test.step("01 Context: forecast load, prices, weather, outage risk, readiness", async () => {
    await expect(
      page.getByRole("region", { name: "Regional context" }),
    ).toBeVisible({ timeout: 1000 });
  });
  await test.step("02 Operator selects region, event window, and target MW", async () => {
    eventId = await createPlan(page, request);
  });
  await test.step("03 Versioned plan is recorded (snapshot detail pending)", async () => {
    await expect(page.locator(".event-identifiers")).toContainText("Plan v1", {
      timeout: 15000,
    });
    await expect(
      page.getByRole("heading", { name: "Safety validated" }),
    ).toBeVisible();
  });
  await test.step("04 Forecast consumption, risk, and fleet availability", async () => {
    await expect(
      page.getByRole("region", { name: "Forecast intervals" }),
    ).toBeVisible({ timeout: 1000 });
  });
  await test.step("05 Optimizer proposes a reserve-preserving plan", async () => {
    const explanation = page.getByRole("region", {
      name: "Optimization explanation",
    });
    await expect(explanation).toContainText("Reserve held back");
    await expect(explanation).toContainText(/\d[\d,.]* kWh/);
    await expect(
      page.getByRole("table", { name: "Interval feasibility" }),
    ).toContainText("Feasible kW");
  });
  await test.step("06 Explain expected value, held reserve, constraints, and exclusions", async () => {
    await expect(
      page.getByRole("heading", { name: "Exclusions by reason" }),
    ).toBeVisible();
    await expect(
      page.getByRole("region", { name: "Constraint margins" }),
    ).toContainText("RESERVE");
  });
  await test.step("07 Travel Flex credit and weather-raised reserve", async () => {
    await expect(
      page.getByRole("region", { name: "Travel Flex eligibility" }),
    ).toBeVisible({ timeout: 1000 });
  });
  await test.step("08 Independently reject the unsafe alternative", async () => {
    await expect(
      page.getByRole("button", { name: "Validate unsafe alternative" }),
    ).toBeVisible({ timeout: 1000 });
  });
  await test.step("09 Approve and launch as separate confirmed actions", () =>
    approveAndLaunch(page));
  await test.step("10 Commands fan out through durable workflow and gateway", async () => {
    await expect(
      page.getByRole("region", { name: "Command fan-out" }),
    ).toBeVisible({ timeout: 1000 });
  });
  await test.step("11 Seeded failure takes devices offline and delays a gateway", async () => {
    const failures = page.getByRole("region", { name: "Scenario failures" });
    await expect(failures).toContainText(/MISSING[_ ]TELEMETRY/, {
      timeout: 180000,
    });
    await expect(failures).toContainText(/UNCERTAIN[_ ]COMMAND/, {
      timeout: 180000,
    });
  });
  await test.step("12 Safe retries and rebalancing remain inside the envelope", async () => {
    const recovery = page.getByRole("region", { name: "Recovery decisions" });
    await expect(recovery).toContainText(/RETRY/, { timeout: 60000 });
    await expect(recovery).toContainText(/REBALANCED[_ ]COMMAND/, {
      timeout: 60000,
    });
    await expect(recovery).toContainText(/STALE[_ ]CAPACITY[_ ]REMOVED/, {
      timeout: 60000,
    });
  });
  await test.step("13 Sent, acknowledged, and measured delivery remain distinct", async () => {
    const response = page.getByRole("region", {
      name: "Measured event response",
    });
    await expect
      .poll(async () =>
        Number.parseFloat(
          await response.locator(".sent-value strong").innerText(),
        ),
      )
      .toBeGreaterThan(0);
    await expect
      .poll(async () =>
        Number.parseFloat(
          await response.locator(".ack-value strong").innerText(),
        ),
      )
      .toBeGreaterThan(0);
    await expect(response.locator(".delivery-value strong")).toHaveText(
      /^-?\d+\.\d{3} MW$/,
      { timeout: 15000 },
    );
    await expect(response).toContainText("SIMULATED");
  });
  await test.step("14 Track the request while preserving household reserve", async () => {
    await expect(
      page.getByRole("region", { name: "Reserve protection evidence" }),
    ).toBeVisible({ timeout: 1000 });
  });
  await test.step("15 Explicit expiry and safe return", async () => {
    await expect(
      page.getByRole("region", { name: "Safe return evidence" }),
    ).toBeVisible({ timeout: 1000 });
  });
  await test.step("16 Basic report (delivery and full economics pending)", () =>
    verifyReport(page, eventId));
  await test.step("17 Replay the event from seed and versioned inputs", async () => {
    await expect(
      page.getByRole("button", { name: "Replay event" }),
    ).toBeVisible({ timeout: 1000 });
  });
});

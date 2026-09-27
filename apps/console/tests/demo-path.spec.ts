import { expect, test, type Page } from "@playwright/test";
import { fromJsonString } from "@bufbuild/protobuf";
import { GetPlanExplanationResponseSchema } from "../src/api/gen/gridos/v1/api_pb";
import { ReserveOverrideReason } from "../src/api/gen/gridos/v1/member_policy_pb";

test.use({
  baseURL: process.env.GRIDOS_DEMO_CONSOLE_URL ?? "http://127.0.0.1:3000",
  timezoneId: "UTC",
});

async function createPlan(page: Page) {
  await page
    .getByRole("link", { name: "Plan a dispatch", exact: true })
    .click();
  const start = Math.ceil((Date.now() + 60000) / 60000) * 60000;
  const end = start + 600000;
  await page.getByLabel("Operating region").selectOption("LZ_AEN");
  await page
    .getByLabel("Start time")
    .fill(new Date(start).toISOString().slice(0, 16));
  await page
    .getByLabel("End time")
    .fill(new Date(end).toISOString().slice(0, 16));
  await page
    .getByLabel("Target power (MW)")
    .fill(process.env.GRIDOS_DEMO_TARGET_MW ?? "20");
  await page
    .getByLabel("Measurement boundary")
    .selectOption("METER_NET_EXPORT");
  await page.getByRole("button", { name: "Create dispatch plan" }).click();
  await expect(
    page.getByRole("heading", { name: "Review the plan" }),
  ).toBeVisible();
  const eventId = new URL(page.url()).pathname.split("/").at(-1) ?? "";
  expect(eventId).not.toBe("new");
  expect(eventId).not.toBe("");
  return { eventId, end };
}

async function reviewFrozenPlan(page: Page, eventId: string) {
  await test.step("03 Inspect the versioned frozen input manifest", async () => {
    await expect(page.locator(".event-identifiers")).toContainText("Plan v1", {
      timeout: 15000,
    });
    await expect(
      page.getByRole("heading", { name: "Safety validated" }),
    ).toBeVisible();
    const manifest = page.getByRole("region", { name: "Frozen plan inputs" });
    await expect(manifest).toContainText("Input snapshot");
    await expect(manifest).toContainText("Eligibility snapshot");
    await expect(manifest).not.toContainText(/pending|unavailable/i);
    await expect(manifest.locator("dd")).not.toHaveCount(0);
  });
  await test.step("04 Inspect household consumption and availability with explicit source gaps", async () => {
    await expect(
      page.getByRole("region", { name: "Forecast intervals" }),
    ).toContainText("5000 sites");
    await expect(
      page.getByRole("region", { name: "Frozen device availability" }),
    ).toContainText("5000");
    await expect(
      page.getByRole("region", { name: "Frozen outage risk" }),
    ).toBeVisible();
    await expect(
      page.getByRole("region", { name: "Unavailable forecast sources" }),
    ).toBeVisible();
  });
  await test.step("05 Optimizer proposes a reserve-preserving plan", async () => {
    const explanation = page.getByRole("region", {
      name: "Optimization explanation",
    });
    await expect(explanation).toContainText("Reserve held back");
    await expect(explanation).toContainText(/\d[\d,.]* kWh/);
    await expect(
      page
        .getByRole("table", { name: "Interval feasibility" })
        .locator("tbody tr"),
    ).not.toHaveCount(0);
  });
  await test.step("06 Explain value, reserve, constraints and exclusions", async () => {
    await expect(
      page.getByRole("heading", { name: "Exclusions by reason" }),
    ).toBeVisible();
    await expect(
      page.getByRole("region", { name: "Constraint margins" }),
    ).toContainText("RESERVE");
    await page
      .getByText("Modeled objective · inspect the value and costs")
      .click();
    await expect(
      page.getByText("Net objective", { exact: true }),
    ).toBeVisible();
  });
  await test.step("07 Show consented Travel Flex credit and a WEATHER reserve override", async () => {
    const response = await page.request.post(
      "/rpc/gridos.v1.DispatchService/GetPlanExplanation",
      {
        headers: { "X-GridOS-Role": "operator" },
        data: { eventId, planVersion: "1" },
      },
    );
    expect(response.ok(), await response.text()).toBe(true);
    const { evidence } = fromJsonString(
      GetPlanExplanationResponseSchema,
      await response.text(),
    );
    const weather = evidence?.reserveBases.find(
      (basis) => basis.overrideReason === ReserveOverrideReason.WEATHER,
    );
    const binding = evidence?.travelFlexBindings[0];
    expect(
      weather,
      "The scenario must freeze a WEATHER override, not just a communications floor",
    ).toBeDefined();
    expect(
      binding,
      "The scenario must freeze a consented Travel Flex window",
    ).toBeDefined();
    if (!weather || !binding)
      throw new Error("Required severe-weather policy seed absent");
    await page.getByLabel("Find reserve device").fill(weather.deviceId);
    const reserve = page.getByRole("region", {
      name: "Household reserve basis",
    });
    await expect(reserve).toContainText("WEATHER");
    await expect(reserve).toContainText(weather.overrideSourceId);
    expect(weather.effectiveReserveKwh).toBeGreaterThanOrEqual(
      weather.overrideFloorKwh!,
    );
    await page.getByLabel("Find Travel Flex window").fill(binding.windowId);
    const travel = page.getByRole("region", {
      name: "Travel Flex eligibility",
    });
    await expect(travel).toContainText(`${binding.creditCents} cents`);
    await expect(travel).toContainText(binding.consentVersion);
    await expect(travel).toContainText(binding.policyVersion);
  });
  await test.step("08 Independently reject the unsafe alternative", async () => {
    await page
      .getByRole("button", { name: "Validate unsafe alternative" })
      .click();
    await expect(
      page.getByRole("heading", { name: "Alternative rejected" }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Safety validated" }),
    ).toBeVisible();
  });
}

async function approveAndLaunch(page: Page) {
  await page.getByLabel("Demo role").selectOption("approver");
  await page.getByRole("button", { name: "Review approval" }).click();
  await page.getByLabel("Type plan version 1").fill("1");
  await page.getByRole("button", { name: "Confirm approval" }).click();
  await expect(
    page.getByRole("heading", { name: "Approved", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Review launch" }).click();
  await page.getByLabel("Type plan version 1").fill("1");
  await page.getByRole("button", { name: "Confirm launch" }).click();
  await expect(
    page.getByRole("heading", {
      name: /^(Sent|Acknowledged or uncertain|Executing|Telemetry verified|Reconciled|Reported)$/,
    }),
  ).toBeVisible({ timeout: 15000 });
  await page.getByRole("link", { name: "Execution", exact: true }).click();
}

async function verifyExecution(page: Page, end: number) {
  await test.step("10 Inspect persisted commands and gateway receipts", async () => {
    const commands = page.getByRole("region", { name: "Command fan-out" });
    await expect(commands).toContainText(
      /SENT|ACKNOWLEDGED|UNCERTAIN|EXECUTING|COMPLETED/,
      { timeout: 15000 },
    );
    await expect(
      commands
        .getByRole("table", { name: "Command records" })
        .locator("tbody tr"),
    ).not.toHaveCount(0);
  });
  await test.step("11 Observe offline telemetry and an uncertain gateway command", async () => {
    const failures = page.getByRole("region", { name: "Recorded failures" });
    await expect(failures).toContainText(/MISSING[_ ]TELEMETRY/, {
      timeout: 180000,
    });
    await expect(failures).toContainText(/UNCERTAIN[_ ]COMMAND/, {
      timeout: 360000,
    });
  });
  await test.step("12 Inspect retry, stale-capacity removal and rebalancing", async () => {
    const recovery = page.getByRole("region", { name: "Recovery decisions" });
    await expect(recovery).toContainText(/RETRY/, { timeout: 60000 });
    await expect(recovery).toContainText(/REBALANCED[_ ]COMMAND/, {
      timeout: 60000,
    });
    await expect(recovery).toContainText(/STALE[_ ]CAPACITY[_ ]REMOVED/, {
      timeout: 60000,
    });
  });
  await test.step("13 Separate sent intent, acknowledgement and measured delivery", async () => {
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
  await test.step("14 Check measured margins against the protected household reserve", async () => {
    const reserve = page.getByRole("region", {
      name: "Reserve protection evidence",
    });
    await expect(reserve).toContainText(
      /[1-9]\d* of [1-9]\d* devices observed · MEASURED/,
      { timeout: 15000 },
    );
    const minimum = Number.parseFloat(await reserve.locator("dd").innerText());
    expect(minimum).toBeGreaterThanOrEqual(0);
    await expect(reserve.getByRole("alert")).toHaveCount(0);
    await expect(reserve).toContainText("Unobserved devices remain unknown");
  });
  await test.step("15 Wait for explicit expiry and recorded zero-setpoint return", async () => {
    await expect
      .poll(() => Date.now(), {
        timeout: Math.max(1000, end - Date.now() + 30000),
        intervals: [5000],
      })
      .toBeGreaterThanOrEqual(end);
    await expect(
      page.getByRole("region", { name: "Safe return evidence" }),
    ).toContainText(/[1-9]\d* zero-setpoint intents/, { timeout: 60000 });
    await expect(
      page.getByRole("region", { name: "Safe return evidence" }),
    ).toContainText("do not confirm the fleet has stopped");
  });
}

async function verifyReport(page: Page, eventId: string) {
  await page.goto(`/events/${eventId}/report`);
  await expect(
    page.getByRole("heading", { name: "Event evidence" }),
  ).toBeVisible();
  const report = page.getByRole("region", {
    name: "Event report",
    exact: true,
  });
  await expect(report).toContainText(eventId);
  for (const label of [
    "Requested power",
    "Acknowledged power",
    "Delivered power",
    "Tracking error",
    "Response latency",
    "Member rewards",
    "Modeled margin",
    "Provenance and versions",
    "Assumptions",
  ])
    await expect(report).toContainText(label);
  await expect(report).toContainText("SIMULATED");
  await expect(report).toContainText("not settled revenue");
}

async function verifyReplay(page: Page) {
  await page.getByRole("button", { name: "Replay event", exact: true }).click();
  const replay = page.getByRole("region", {
    name: "Event replay",
    exact: true,
  });
  await expect(replay).toContainText("IDENTICAL", { timeout: 30000 });
  await expect(replay).toContainText("Input snapshot");
  await expect(replay).toContainText("Eligibility snapshot");
  await expect(replay).toContainText("Fleet SHA-256");
  const slider = page.getByRole("slider", { name: "Replay position" });
  await expect(slider).toBeVisible();
  const maximum = await slider.getAttribute("max");
  expect(Number(maximum)).toBeGreaterThan(0);
  await slider.fill(maximum!);
  const finalTime = await page
    .getByLabel("Replay time", { exact: true })
    .innerText();
  await expect(
    page.getByRole("region", { name: "Historical geography clock" }),
  ).toContainText(finalTime);
  await slider.fill("0");
  await expect(page.getByLabel("Replay time", { exact: true })).not.toHaveText(
    finalTime,
  );
}

test("seventeen-step severe-weather operating loop against the live demo", async ({
  page,
}, testInfo) => {
  test.setTimeout(960000);
  await page.goto("/fleet");
  await test.step("01 Inspect Austin markets, weather, outage history and fleet readiness", async () => {
    const context = page.getByRole("region", { name: "Austin conditions" });
    await expect(context).toBeVisible();
    await context
      .getByText("Inspect Austin markets · LZ_AEN and SOUTH_C")
      .click();
    await expect(
      context.getByRole("table", {
        name: "Day-ahead reference prices",
        exact: true,
      }),
    ).toContainText("LZ_AEN");
    await expect(
      context.getByRole("table", {
        name: "Reference system load",
        exact: true,
      }),
    ).toContainText("SOUTH_C");
    await context
      .getByText("Inspect weather and historical outage evidence")
      .click();
    await expect(
      context
        .getByRole("table", { name: "Austin weather forecasts", exact: true })
        .locator("tbody tr"),
    ).not.toHaveCount(0);
    await expect(
      context.getByRole("heading", { name: "Travis County outage history" }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Availability", exact: true }),
    ).toBeVisible();
  });
  const plan =
    await test.step("02 Select region, event window and target power", () =>
      createPlan(page));
  console.log(`DEMO EVENT ${plan.eventId}`);
  await reviewFrozenPlan(page, plan.eventId);
  await test.step("09 Approve and launch as separate confirmed actions", () =>
    approveAndLaunch(page));
  await verifyExecution(page, plan.end);
  await page.screenshot({
    path: testInfo.outputPath("demo-execution.png"),
    fullPage: true,
  });
  await test.step("16 Inspect delivery accounting, reserve, economics and lineage", () =>
    verifyReport(page, plan.eventId));
  await test.step("17 Reproduce the plan and scrub the shared replay clock", () =>
    verifyReplay(page));
  await page.screenshot({
    path: testInfo.outputPath("demo-replay.png"),
    fullPage: true,
  });
});

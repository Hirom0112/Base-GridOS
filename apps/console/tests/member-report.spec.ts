import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { openEvidence, recordedApi } from "./recorded-api";

for (const width of [390, 1440]) {
  test(`member household is isolated and usable at ${width}`, async ({
    page,
  }, testInfo) => {
    await recordedApi(page);
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/fleet");
    await page
      .getByRole("combobox", { name: "Demo role" })
      .selectOption("member");
    await page
      .getByLabel("Local member principal")
      .fill("member-9d5ecb7e1fda8fcce5d0");
    await page
      .getByLabel("Site ID", { exact: true })
      .fill("site_9d5ecb7e1fda8fcce5d0");
    const request = page.waitForRequest(
      "**/gridos.v1.MemberService/GetMemberStatus",
    );
    await page.getByRole("button", { name: "Open household" }).click();
    expect((await request).headers()["x-gridos-member-id"]).toBe(
      "member-9d5ecb7e1fda8fcce5d0",
    );
    await expect(page.getByRole("heading", { name: "On grid" })).toBeVisible();
    await expect(
      page.getByRole("region", { name: "Plan and reserve" }),
    ).toContainText("65%");
    await expect(
      page.getByText("energy anomaly signal", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Installed power", { exact: true }),
    ).toHaveCount(0);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    expect(
      (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze())
        .violations,
    ).toEqual([]);
    await page.screenshot({
      path: testInfo.outputPath(`member-${width}.png`),
      fullPage: true,
    });
  });

  test(`report and comparison preserve evidence at ${width}`, async ({
    page,
  }, testInfo) => {
    await recordedApi(page);
    await page.route("**/gridos.v1.DispatchService/GetEvent", async (route) => {
      const fixture = JSON.parse(
        await readFile(
          "../../testdata/fixtures/api/DispatchService/GetEvent.json",
          "utf8",
        ),
      );
      fixture.event.eventId = "event-report-a";
      await route.fulfill({ json: fixture });
    });
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/events/event-report-a/report");
    const report = page.getByRole("region", {
      name: "Event report",
      exact: true,
    });
    await expect(report).toContainText("-3.25 USD");
    await expect(report).toContainText("delivered_energy_unavailable");
    await openEvidence(page);
    await page.getByLabel("Event B", { exact: true }).fill("event-report-b");
    await page.getByRole("button", { name: "Compare reports" }).click();
    await expect(
      page.getByRole("table", { name: "Report differences" }),
    ).toContainText("member_rewards_cents");
    await expect(
      page.getByRole("button", { name: "Replay event", exact: true }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    expect(
      (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze())
        .violations,
    ).toEqual([]);
    await report.screenshot({
      path: testInfo.outputPath(`report-${width}.png`),
    });
  });
}

import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import AxeBuilder from "@axe-core/playwright";
import { recordedApi } from "./recorded-api";

for (const width of [390, 1440]) {
  test(`approval and launch require separate server-backed confirmations at ${width}`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await recordedApi(page);
    const responses = await Promise.all(
      ["GetEvent", "ApproveEvent", "LaunchEvent"].map(
        async (name) =>
          JSON.parse(
            await readFile(
              resolve(
                process.cwd(),
                `../../testdata/fixtures/api/DispatchService/${name}.json`,
              ),
              "utf8",
            ),
          ) as Record<string, unknown>,
      ),
    );
    let event = responses[0];
    const commands: string[] = [];
    await page.route("**/rpc/gridos.v1.DispatchService/*", async (route) => {
      const method = new URL(route.request().url()).pathname.split("/").at(-1);
      if (method === "CreateEventRequest") return route.fallback();
      if (method === "ApproveEvent") {
        commands.push(method);
        event = { ...event, ...responses[1] };
      }
      if (method === "LaunchEvent") {
        commands.push(method);
        event = { ...event, ...responses[2] };
      }
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify(event),
      });
    });
    await page.goto("/dispatch/new");
    await page.getByLabel("Start time").fill("2026-09-27T18:00");
    await page.getByLabel("End time").fill("2026-09-27T19:00");
    await page.getByLabel("Target power (MW)").fill("0.001");
    await page.getByRole("button", { name: "Create dispatch plan" }).click();
    await expect(
      page.getByRole("heading", { name: "Safety validated" }),
    ).toBeVisible();
    await expect(
      page.getByText("Approver role required", { exact: false }),
    ).toBeVisible();
    await page.getByLabel("Demo role").selectOption("approver");
    await page.getByRole("button", { name: "Review approval" }).click();
    await page.getByLabel("Type plan version 1").fill("2");
    await page.getByRole("button", { name: "Confirm approval" }).click();
    await expect(page.getByRole("alert")).toContainText("exact plan version");
    expect(commands).toEqual([]);
    await page.getByLabel("Type plan version 1").fill("1");
    expect(
      (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze())
        .violations,
    ).toEqual([]);
    await page.screenshot({
      path: `test-results/approval-dialog-${width}.png`,
    });
    await page.getByRole("button", { name: "Confirm approval" }).click();
    await expect(
      page.getByRole("heading", { name: "Approved", exact: true }),
    ).toBeVisible();
    expect(commands).toEqual(["ApproveEvent"]);
    await expect(page.getByText("No launch record returned")).toBeVisible();
    await page.getByRole("button", { name: "Review launch" }).click();
    await page.getByLabel("Type plan version 1").fill("1");
    await page.screenshot({ path: `test-results/launch-dialog-${width}.png` });
    await page.getByRole("button", { name: "Confirm launch" }).click();
    await expect(
      page.getByRole("heading", {
        name: "Acknowledged or uncertain",
        exact: true,
      }),
    ).toBeVisible();
    expect(commands).toEqual(["ApproveEvent", "LaunchEvent"]);
    await page.getByRole("link", { name: "Execution", exact: true }).click();
    await expect(
      page.getByText("Awaiting measured delivery evidence"),
    ).toBeVisible();
    await page.screenshot({
      path: `test-results/execution-${width}.png`,
      fullPage: true,
    });
    await page.getByRole("link", { name: "Report", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "Event evidence" }),
    ).toBeVisible();
    await expect(page.getByText(/Verified delivery, replay/)).toBeVisible();
    expect(
      (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze())
        .violations,
    ).toEqual([]);
    await page.screenshot({
      path: `test-results/report-${width}.png`,
      fullPage: true,
    });
    await page
      .getByRole("link", { name: "01 Observe Understand the fleet" })
      .click();
    await expect(
      page.getByRole("link", { name: "04 Approve Review and authorize" }),
    ).toBeVisible();
  });
}

import { expect, test } from "@playwright/test";
import { recordedApi } from "./recorded-api";

test("comparison route is distinct from event detail and sends selected versions", async ({
  page,
}) => {
  await recordedApi(page);
  await page.goto("/fleet");
  await page
    .getByRole("link", { name: "Compare reports", exact: true })
    .click();
  await expect(page).toHaveURL(/\/events\/compare$/);
  await expect(
    page.getByRole("heading", { name: "Compare event evidence" }),
  ).toBeVisible();
  await page.getByLabel("Event A", { exact: true }).fill("event-report-a");
  await page.getByLabel("Event B", { exact: true }).fill("event-report-b");
  await page.getByLabel("Plan version A", { exact: true }).fill("1");
  await page.getByLabel("Plan version B", { exact: true }).fill("2");
  const request = page.waitForRequest(
    "**/gridos.v1.ReportService/CompareEventReports",
  );
  await page
    .getByRole("button", { name: "Compare reports", exact: true })
    .click();
  expect((await request).postDataJSON()).toEqual({
    eventIdA: "event-report-a",
    eventIdB: "event-report-b",
    planVersionA: "1",
    planVersionB: "2",
  });
  await expect(
    page.getByRole("table", { name: "Report differences" }),
  ).toContainText("margin.value_usd");
});

test("member route opens a household view only for a member session", async ({
  page,
}) => {
  await recordedApi(page);
  await page.goto("/member");
  await expect(
    page.getByRole("heading", { name: "Member access", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Open household" }),
  ).toHaveCount(0);
  await page
    .getByRole("combobox", { name: "Demo role" })
    .selectOption("member");
  await expect(
    page.getByRole("heading", { name: "Home energy", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("navigation", { name: "Operating loop" }),
  ).toHaveCount(0);
});

import { expect, test } from "@playwright/test";
import { recordedApi } from "./recorded-api";

test("matching event evidence exposes the selected plan", async ({ page }) => {
  await recordedApi(page);
  await page.goto("/dispatch/event_austin_wave2_live_0002");
  await page.getByLabel("Demo role").selectOption("approver");
  await expect(page.locator(".event-identifiers")).toContainText("Plan v1");
  await expect(
    page.getByRole("button", { name: "Review approval" }),
  ).toBeVisible();
});

test("foreign event evidence cannot authorize the selected event", async ({
  page,
}) => {
  await recordedApi(page);
  await page.goto("/dispatch/a-different-event");
  await page.getByLabel("Demo role").selectOption("approver");
  await expect(page.getByRole("alert")).toContainText(
    "Event evidence does not match the selected event",
  );
  await expect(
    page.getByRole("button", { name: "Review approval" }),
  ).toHaveCount(0);
  await expect(page.locator(".event-identifiers")).toHaveCount(0);
});

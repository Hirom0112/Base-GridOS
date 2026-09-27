import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { openEvidence, recordedApi } from "./recorded-api";

for (const width of [390, 1440]) {
  for (const theme of ["dark", "light"]) {
    test(`audit records at ${width} in ${theme}`, async ({ page }) => {
      await recordedApi(page);
      await page.setViewportSize({ width, height: 1000 });
      await page.route(
        "**/rpc/gridos.v1.EventsService/GetEventTimeline",
        async (route) => {
          await route.fulfill({
            contentType: "application/json",
            body: JSON.stringify({
              entries: [
                {
                  sequence: "1",
                  occurredAt: "2026-09-26T12:00:00Z",
                  actorId: "workflow",
                  action: "EVENT_STATE_TRANSITIONED",
                  previousState: "DISPATCH_EVENT_STATE_APPROVED",
                  state: "DISPATCH_EVENT_STATE_COMMANDS_PERSISTED",
                  reason: "Approved plan persisted",
                },
                {
                  sequence: "2",
                  occurredAt: "2026-09-26T12:00:01Z",
                  actorId: "gateway",
                  action: "RETRY_SCHEDULED",
                  reason: "Gateway timeout; bounded retry scheduled",
                },
              ],
            }),
          });
        },
      );
      await page.goto("/events/event_austin_wave2_live_0002");
      if (theme === "light")
        await page.getByRole("button", { name: "Use light theme" }).click();
      await openEvidence(page);
      await expect(
        page
          .getByRole("list", { name: "Server audit timeline" })
          .getByRole("listitem"),
      ).toHaveCount(2);
      await expect(
        page.getByText("Approved → Commands persisted"),
      ).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      expect(
        (
          await new AxeBuilder({ page })
            .withTags(["wcag2a", "wcag2aa"])
            .analyze()
        ).violations,
      ).toEqual([]);
      await page
        .locator(".event-panel")
        .screenshot({ path: `test-results/audit-${theme}-${width}.png` });
    });
  }
}

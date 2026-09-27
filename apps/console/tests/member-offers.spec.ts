import { readFile } from "node:fs/promises";
import { create, fromJsonString, toJsonString } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import {
  GetMemberStatusResponseSchema,
  ListMemberOffersResponseSchema,
  MemberOfferTermsSchema,
  PresentOfferRequestSchema,
  PresentOfferResponseSchema,
  SelectResiliencePlanRequestSchema,
  SelectResiliencePlanResponseSchema,
  SelectedMemberPlanSchema,
  ScheduleTravelFlexRequestSchema,
  ScheduleTravelFlexResponseSchema,
  ScheduledTravelFlexWindowSchema,
  EndTravelFlexEarlyRequestSchema,
  EndTravelFlexEarlyResponseSchema,
} from "../src/api/gen/gridos/v1/member_pb";
import { recordedApi } from "./recorded-api";

async function memberData() {
  const status = fromJsonString(
    GetMemberStatusResponseSchema,
    await readFile(
      "../../testdata/fixtures/api/MemberService/GetMemberStatus.json",
      "utf8",
    ),
  );
  const terms = create(MemberOfferTermsSchema, {
    kind: 1,
    market: "ERCOT",
    catalogVersion: "catalog-1",
    memberPlanId: "balanced",
    displayName: "Balanced",
    reserveFloorPercent: 30,
    policyVersion: "policy-2",
    contractVersion: "contract-1",
    consentVersion: "consent-1",
    consentText: "SIMULATED plan consent",
    priceText: "SIMULATED stored plan price",
    effectiveAt: timestampFromDate(new Date("2020-01-01")),
    expiresAt: timestampFromDate(new Date("2100-01-01")),
  });
  const offers = create(ListMemberOffersResponseSchema, {
    offers: [
      terms,
      {
        ...terms,
        kind: 2,
        temporaryReservePercent: 20,
        creditType: 1,
        fixedCreditCents: 500n,
        consentVersion: "travel-consent-2",
        consentText: "SIMULATED travel consent",
      },
    ],
    travelFlexWindows: [
      {
        windowId: "active-window",
        startTime: timestampFromDate(new Date("2020-01-01")),
        endTime: timestampFromDate(new Date("2100-01-01")),
        timezone: "America/Chicago",
        temporaryReservePercent: 20,
        creditType: 1,
        fixedCreditCents: 500n,
        consentVersion: "historic-consent",
        earlyReturnAction: 2,
      },
    ],
  });
  return { status, terms, offers };
}

async function memberApi(page: Page) {
  await recordedApi(page);
  const { status, terms, offers } = await memberData();
  const selections: string[] = [];
  await page.route("**/gridos.v1.MemberService/GetMemberStatus", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: toJsonString(GetMemberStatusResponseSchema, status),
    }),
  );
  await page.route("**/gridos.v1.MemberService/ListMemberOffers", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: toJsonString(ListMemberOffersResponseSchema, offers),
    }),
  );
  await page.route("**/gridos.v1.MemberService/PresentOffer", (route) => {
    const input = fromJsonString(
      PresentOfferRequestSchema,
      route.request().postData()!,
    );
    expect(input.memberId).toBe(status.memberId);
    return route.fulfill({
      contentType: "application/json",
      body: toJsonString(
        PresentOfferResponseSchema,
        create(PresentOfferResponseSchema, {
          offer: {
            ...input,
            $typeName: undefined,
            offerId: `offer-${input.kind}`,
          },
        }),
      ),
    });
  });
  await page.route(
    "**/gridos.v1.MemberService/SelectResiliencePlan",
    (route) => {
      selections.push(route.request().postData()!);
      if (selections.length === 1)
        return route.fulfill({
          status: 503,
          json: { code: "unavailable", message: "Receipt interrupted" },
        });
      const input = fromJsonString(
        SelectResiliencePlanRequestSchema,
        route.request().postData()!,
      );
      expect(input.policyVersion).toBe(terms.policyVersion);
      expect(input.consentVersion).toBe(terms.consentVersion);
      expect(input.explanationShown).toContain("30%");
      status.currentPlan = create(SelectedMemberPlanSchema, {
        ...terms,
        $typeName: undefined,
        selectionId: "selected-1",
        offerId: input.offerId,
        effectiveAt: input.effectiveAt,
        termsKnown: true,
      });
      return route.fulfill({
        contentType: "application/json",
        body: toJsonString(
          SelectResiliencePlanResponseSchema,
          create(SelectResiliencePlanResponseSchema, {
            plan: status.currentPlan,
          }),
        ),
      });
    },
  );
  await page.route("**/gridos.v1.MemberService/ScheduleTravelFlex", (route) => {
    const input = fromJsonString(
      ScheduleTravelFlexRequestSchema,
      route.request().postData()!,
    );
    expect(input.policyVersion).toBe("policy-2");
    expect(input.consentVersion).toBe("travel-consent-2");
    expect(input.temporaryReservePercent).toBe(20);
    offers.travelFlexWindows.push(
      create(ScheduledTravelFlexWindowSchema, {
        ...input,
        $typeName: undefined,
        windowId: "scheduled-window",
      }),
    );
    return route.fulfill({
      contentType: "application/json",
      body: toJsonString(
        ScheduleTravelFlexResponseSchema,
        create(ScheduleTravelFlexResponseSchema, {
          ...input,
          $typeName: undefined,
          windowId: "scheduled-window",
        }),
      ),
    });
  });
  await page.route("**/gridos.v1.MemberService/EndTravelFlexEarly", (route) => {
    const input = fromJsonString(
      EndTravelFlexEarlyRequestSchema,
      route.request().postData()!,
    );
    expect(input.consentVersion).toBe("historic-consent");
    expect(input.windowId).toBe("active-window");
    offers.travelFlexWindows[0]!.cancelledAt = input.returnedAt;
    return route.fulfill({
      contentType: "application/json",
      body: toJsonString(
        EndTravelFlexEarlyResponseSchema,
        create(EndTravelFlexEarlyResponseSchema, {
          ...input,
          $typeName: undefined,
        }),
      ),
    });
  });
  return { status, selections };
}

for (const width of [390, 1440]) {
  test(`member reviews, selects, schedules and returns at ${width}`, async ({
    page,
  }, testInfo) => {
    const { status, selections } = await memberApi(page);
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/member");
    await page
      .getByRole("combobox", { name: "Demo role" })
      .selectOption("member");
    await page.getByLabel("Local member principal").fill(status.memberId);
    await page.getByLabel("Site ID", { exact: true }).fill(status.siteId);
    await page.getByRole("button", { name: "Open household" }).click();
    await page.getByRole("button", { name: "View Balanced plan" }).click();
    await page.getByRole("button", { name: "Review offer" }).click();
    await expect(
      page.getByRole("button", { name: "Confirm plan" }),
    ).toBeDisabled();
    await page
      .getByRole("checkbox", { name: "I agree to the displayed plan terms." })
      .check();
    await page.getByRole("button", { name: "Confirm plan" }).click();
    await expect(page.getByRole("alert")).toContainText("Outcome unknown");
    await page.getByRole("button", { name: "Retry same selection" }).click();
    await expect(page.getByText(/Plan selection recorded/)).toBeVisible();
    expect(selections).toHaveLength(2);
    expect(selections[0]).toBe(selections[1]);
    await page.getByRole("button", { name: "Browse offers" }).click();
    await page
      .getByRole("button", { name: "View Balanced Travel Flex" })
      .click();
    await page.getByRole("button", { name: "Review offer" }).click();
    await page.getByLabel("Travel starts").fill("2090-09-28T10:00");
    await page.getByLabel("Travel ends").fill("2090-09-29T12:00");
    await page.getByLabel("Household timezone").fill("America/Chicago");
    await page
      .getByRole("checkbox", {
        name: "I agree to the displayed Travel Flex terms and reserve change.",
      })
      .check();
    await page.getByRole("button", { name: "Confirm Travel Flex" }).click();
    await expect(page.getByText(/Travel Flex scheduled/)).toBeVisible();
    const active = page
      .getByText(/On early return: Restore maximum reserve/)
      .locator("..");
    await active.getByRole("checkbox").check();
    await active.getByRole("button", { name: "Confirm early return" }).click();
    await expect(page.getByText(/Early return recorded/)).toBeVisible();
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
      path: testInfo.outputPath(`member-offers-${width}.png`),
      fullPage: true,
    });
  });
}

import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, expect, test, vi } from "vitest";
import {
  ListMemberOffersResponseSchema,
  SelectedMemberPlanSchema,
} from "../api/gen/gridos/v1/member_pb";
import MemberOffers from "./offers";

const session = vi.hoisted(() => ({
  identity: { role: "member" },
  client: { member: { listMemberOffers: vi.fn() } },
}));
vi.mock("../api/auth", () => ({ useSession: () => session }));
const currentPlan = create(SelectedMemberPlanSchema, {
  memberPlanId: "balanced",
  catalogVersion: "catalog-1",
  policyVersion: "old-policy",
  reserveFloorPercent: 30,
});
function mount() {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <MemberOffers memberId="member-1" currentPlan={currentPlan} />
    </QueryClientProvider>,
  );
}
beforeEach(() => session.client.member.listMemberOffers.mockReset());

test("offers retain policy linkage and incompatible travel cannot be selected", async () => {
  session.client.member.listMemberOffers.mockResolvedValue(
    create(ListMemberOffersResponseSchema, {
      offers: [
        {
          kind: 2,
          market: "ERCOT",
          catalogVersion: "catalog-1",
          memberPlanId: "balanced",
          displayName: "Balanced",
          policyVersion: "new-policy",
          contractVersion: "contract-1",
          consentVersion: "consent-1",
          consentText: "Consent",
          priceText: "Price",
          reserveFloorPercent: 30,
          temporaryReservePercent: 20,
          creditType: 1,
          fixedCreditCents: 500n,
          effectiveAt: timestampFromDate(new Date("2020-01-01")),
          expiresAt: timestampFromDate(new Date("2100-01-01")),
        },
      ],
    }),
  );
  mount();
  expect(
    await screen.findByText(/Choose a matching plan before scheduling/),
  ).toBeVisible();
  expect(
    screen.getByRole("button", { name: "View Balanced Travel Flex" }),
  ).toBeDisabled();
});

test("empty offers remain explicitly unavailable", async () => {
  session.client.member.listMemberOffers.mockResolvedValue(
    create(ListMemberOffersResponseSchema),
  );
  mount();
  expect(
    await screen.findByText("No offers are available for this household."),
  ).toBeVisible();
  expect(
    screen.queryByRole("button", { name: /View .* plan/ }),
  ).not.toBeInTheDocument();
});

test("the current plan is identified while a different policy remains selectable", async () => {
  const offer = {
    kind: 1,
    market: "ERCOT",
    catalogVersion: "catalog-1",
    memberPlanId: "balanced",
    displayName: "Balanced",
    policyVersion: "old-policy",
    contractVersion: "contract-1",
    consentVersion: "consent-1",
    consentText: "Consent",
    priceText: "Price",
    reserveFloorPercent: 30,
    effectiveAt: timestampFromDate(new Date("2020-01-01")),
    expiresAt: timestampFromDate(new Date("2100-01-01")),
  };
  session.client.member.listMemberOffers.mockResolvedValue(
    create(ListMemberOffersResponseSchema, {
      offers: [
        offer,
        {
          ...offer,
          memberPlanId: "maximum",
          displayName: "Maximum",
          policyVersion: "maximum-policy",
        },
      ],
    }),
  );
  mount();
  expect(
    await screen.findByRole("button", { name: "Current plan: Balanced" }),
  ).toBeDisabled();
  expect(
    screen.getByRole("button", { name: "View Maximum plan" }),
  ).toBeEnabled();
});

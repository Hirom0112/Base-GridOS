import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, test, vi } from "vitest";
import {
  MemberOfferTermsSchema,
  MemberOfferSchema,
} from "../api/gen/gridos/v1/member_pb";
import { OfferReview } from "./offer-review";

const session = vi.hoisted(() => ({
  client: { member: { presentOffer: vi.fn(), selectResiliencePlan: vi.fn() } },
}));
vi.mock("../api/auth", () => ({ useSession: () => session }));
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
  consentText: "Exact consent",
  priceText: "Exact price",
  effectiveAt: timestampFromDate(new Date("2020-01-01T00:00:00Z")),
  expiresAt: timestampFromDate(new Date("2100-01-01T00:00:00Z")),
});
beforeEach(() => session.client.member.presentOffer.mockReset());

test("reviews exact terms and preserves the recorded offer intent across retries", async () => {
  session.client.member.presentOffer.mockRejectedValueOnce(
    new Error("Timeout"),
  );
  session.client.member.presentOffer.mockResolvedValueOnce({
    offer: create(MemberOfferSchema, {
      ...terms,
      memberId: "member-1",
      offerId: "offer-1",
    }),
  });
  render(
    <OfferReview memberId="member-1" terms={terms} onConfirmed={vi.fn()} />,
  );
  expect(screen.getByText("Exact price")).toBeVisible();
  expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Review offer" }));
  await userEvent.click(
    await screen.findByRole("button", { name: "Retry same offer" }),
  );
  expect(await screen.findByRole("checkbox")).toBeVisible();
  expect(session.client.member.presentOffer.mock.calls[0]![0]).toEqual(
    session.client.member.presentOffer.mock.calls[1]![0],
  );
  expect(session.client.member.selectResiliencePlan).not.toHaveBeenCalled();
});

test("a foreign offer receipt never unlocks consent", async () => {
  session.client.member.presentOffer.mockResolvedValueOnce({
    offer: create(MemberOfferSchema, {
      ...terms,
      memberId: "another-member",
      offerId: "offer-1",
    }),
  });
  render(
    <OfferReview memberId="member-1" terms={terms} onConfirmed={vi.fn()} />,
  );
  await userEvent.click(screen.getByRole("button", { name: "Review offer" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Offer review is not confirmed",
  );
  expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
});

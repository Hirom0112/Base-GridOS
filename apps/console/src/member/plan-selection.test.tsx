import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, test, vi } from "vitest";
import {
  MemberOfferTermsSchema,
  SelectedMemberPlanSchema,
  type SelectResiliencePlanRequest,
} from "../api/gen/gridos/v1/member_pb";
import { PlanSelection } from "./plan-selection";

const session = vi.hoisted(() => ({
  client: { member: { selectResiliencePlan: vi.fn() } },
}));
vi.mock("../api/auth", () => ({ useSession: () => session }));
const terms = create(MemberOfferTermsSchema, {
  kind: 1,
  memberPlanId: "balanced",
  catalogVersion: "catalog-1",
  policyVersion: "policy-2",
  market: "ERCOT",
  reserveFloorPercent: 30,
  consentText: "The exact consent",
  consentVersion: "consent-1",
  expiresAt: timestampFromDate(new Date("2100-01-01T00:00:00Z")),
});

beforeEach(() => session.client.member.selectResiliencePlan.mockReset());

test("selection requires consent and retries the same intent after an uncertain outcome", async () => {
  const refreshed = vi.fn();
  session.client.member.selectResiliencePlan.mockRejectedValueOnce(
    new Error("Connection lost"),
  );
  session.client.member.selectResiliencePlan.mockImplementationOnce(
    async (request: SelectResiliencePlanRequest) => ({
      plan: create(SelectedMemberPlanSchema, {
        ...terms,
        $typeName: undefined,
        offerId: "offer-1",
        selectionId: "selection-1",
        termsKnown: true,
        effectiveAt: request.effectiveAt,
      }),
    }),
  );
  render(
    <PlanSelection
      terms={terms}
      memberId="member-1"
      offerId="offer-1"
      onConfirmed={refreshed}
      onDismiss={vi.fn()}
    />,
  );
  expect(screen.getByRole("button", { name: "Confirm plan" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Cancel review" })).toBeEnabled();
  await userEvent.click(screen.getByRole("checkbox"));
  await userEvent.click(screen.getByRole("button", { name: "Confirm plan" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Outcome unknown");
  expect(refreshed).not.toHaveBeenCalled();
  expect(
    screen.queryByRole("button", { name: "Cancel review" }),
  ).not.toBeInTheDocument();
  await userEvent.click(
    screen.getByRole("button", { name: "Retry same selection" }),
  );
  expect(await screen.findByRole("status")).toHaveTextContent(
    "Plan selection recorded",
  );
  const calls = session.client.member.selectResiliencePlan.mock.calls;
  expect(calls[0]![0]).toEqual(calls[1]![0]);
  expect(calls[0]![0]).toMatchObject({
    memberId: "member-1",
    offerId: "offer-1",
    policyVersion: "policy-2",
    consentText: terms.consentText,
    consentVersion: terms.consentVersion,
  });
  expect(refreshed).toHaveBeenCalledTimes(1);
});

test("a mismatched receipt cannot confirm a selection", async () => {
  session.client.member.selectResiliencePlan.mockResolvedValue({
    plan: create(SelectedMemberPlanSchema, {
      ...terms,
      $typeName: undefined,
      offerId: "other",
      selectionId: "selection-1",
      termsKnown: true,
    }),
  });
  const refreshed = vi.fn();
  render(
    <PlanSelection
      terms={terms}
      memberId="member-1"
      offerId="offer-1"
      onConfirmed={refreshed}
    />,
  );
  await userEvent.click(screen.getByRole("checkbox"));
  await userEvent.click(screen.getByRole("button", { name: "Confirm plan" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Outcome unknown");
  expect(refreshed).not.toHaveBeenCalled();
});

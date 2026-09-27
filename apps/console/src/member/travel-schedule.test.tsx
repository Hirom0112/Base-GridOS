import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, test, vi } from "vitest";
import {
  MemberOfferTermsSchema,
  type ScheduleTravelFlexRequest,
} from "../api/gen/gridos/v1/member_pb";
import { TravelSchedule } from "./travel-schedule";

const session = vi.hoisted(() => ({
  client: { member: { scheduleTravelFlex: vi.fn() } },
}));
vi.mock("../api/auth", () => ({ useSession: () => session }));
const terms = create(MemberOfferTermsSchema, {
  kind: 2,
  temporaryReservePercent: 20,
  creditType: 1,
  fixedCreditCents: 500n,
  consentText: "Travel consent",
  consentVersion: "travel-consent-2",
  policyVersion: "policy-3",
  expiresAt: timestampFromDate(new Date("2100-01-01T00:00:00Z")),
});
beforeEach(() => session.client.member.scheduleTravelFlex.mockReset());

async function fill(start = "2090-09-28T10:00") {
  await userEvent.type(screen.getByLabelText("Travel starts"), start);
  await userEvent.type(
    screen.getByLabelText("Travel ends"),
    "2090-09-29T12:00",
  );
  await userEvent.clear(screen.getByLabelText("Household timezone"));
  await userEvent.type(
    screen.getByLabelText("Household timezone"),
    "America/Chicago",
  );
  await userEvent.click(screen.getByRole("checkbox"));
}

test("schedules exact offered terms and retries the same window after a timeout", async () => {
  const confirmed = vi.fn();
  session.client.member.scheduleTravelFlex.mockRejectedValueOnce(
    new Error("Timeout"),
  );
  session.client.member.scheduleTravelFlex.mockImplementationOnce(
    async (request: ScheduleTravelFlexRequest) => ({
      ...request,
      windowId: "window-1",
    }),
  );
  render(
    <TravelSchedule
      terms={terms}
      memberId="member-1"
      offerId="offer-1"
      onConfirmed={confirmed}
    />,
  );
  expect(
    screen.getByRole("button", { name: "Confirm Travel Flex" }),
  ).toBeDisabled();
  await fill();
  await userEvent.click(
    screen.getByRole("button", { name: "Confirm Travel Flex" }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent("Outcome unknown");
  expect(screen.getByLabelText("Travel starts")).toBeDisabled();
  await userEvent.click(
    screen.getByRole("button", { name: "Retry same travel window" }),
  );
  expect(await screen.findByRole("status")).toHaveTextContent(
    "Travel Flex scheduled",
  );
  const calls = session.client.member.scheduleTravelFlex.mock.calls;
  expect(calls[0]![0]).toEqual(calls[1]![0]);
  expect(calls[0]![0]).toMatchObject({
    memberId: "member-1",
    offerId: "offer-1",
    policyVersion: "policy-3",
    consentVersion: "travel-consent-2",
    temporaryReservePercent: 20,
    fixedCreditCents: 500n,
    timezone: "America/Chicago",
    earlyReturnAction: 1,
  });
  expect(confirmed).toHaveBeenCalledTimes(1);
});

test("past travel cannot submit and a mismatched receipt cannot confirm", async () => {
  const confirmed = vi.fn();
  render(
    <TravelSchedule
      terms={terms}
      memberId="member-1"
      offerId="offer-1"
      onConfirmed={confirmed}
    />,
  );
  await fill("2020-09-28T10:00");
  await userEvent.click(
    screen.getByRole("button", { name: "Confirm Travel Flex" }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent("future");
  expect(session.client.member.scheduleTravelFlex).not.toHaveBeenCalled();
  await userEvent.clear(screen.getByLabelText("Travel starts"));
  await userEvent.type(
    screen.getByLabelText("Travel starts"),
    "2090-09-28T10:00",
  );
  session.client.member.scheduleTravelFlex.mockResolvedValue({
    windowId: "window-1",
    temporaryReservePercent: 0,
  });
  await userEvent.click(
    screen.getByRole("button", { name: "Confirm Travel Flex" }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent("Outcome unknown");
  expect(confirmed).not.toHaveBeenCalled();
});

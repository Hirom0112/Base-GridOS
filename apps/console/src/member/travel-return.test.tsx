import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import {
  ScheduledTravelFlexWindowSchema,
  type EndTravelFlexEarlyRequest,
} from "../api/gen/gridos/v1/member_pb";
import { TravelReturn } from "./travel-return";

const session = vi.hoisted(() => ({
  client: { member: { endTravelFlexEarly: vi.fn() } },
}));
vi.mock("../api/auth", () => ({ useSession: () => session }));

test("future windows cannot request an early return", () => {
  const window = create(ScheduledTravelFlexWindowSchema, {
    windowId: "future-window",
    consentVersion: "original-consent",
    earlyReturnAction: 1,
    startTime: timestampFromDate(new Date("2090-01-01")),
    endTime: timestampFromDate(new Date("2100-01-01")),
  });
  render(
    <TravelReturn memberId="member-1" window={window} onConfirmed={vi.fn()} />,
  );
  expect(
    screen.getByText(
      "Early return is available while this travel window is active.",
    ),
  ).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "Confirm early return" }),
  ).not.toBeInTheDocument();
});

test("early return confirms the stored action and reuses the historical consent and intent", async () => {
  const window = create(ScheduledTravelFlexWindowSchema, {
    windowId: "window-1",
    consentVersion: "historic-consent-1",
    earlyReturnAction: 2,
    startTime: timestampFromDate(new Date("2020-01-01")),
    endTime: timestampFromDate(new Date("2100-01-01")),
  });
  const confirmed = vi.fn();
  session.client.member.endTravelFlexEarly.mockRejectedValueOnce(
    new Error("Timeout"),
  );
  session.client.member.endTravelFlexEarly.mockImplementationOnce(
    async (request: EndTravelFlexEarlyRequest) => ({
      windowId: request.windowId,
      returnedAt: request.returnedAt,
    }),
  );
  render(
    <TravelReturn
      memberId="member-1"
      window={window}
      onConfirmed={confirmed}
    />,
  );
  expect(
    screen.getByRole("button", { name: "Confirm early return" }),
  ).toBeDisabled();
  expect(screen.getByText(/Restore maximum reserve/)).toBeVisible();
  await userEvent.click(screen.getByRole("checkbox"));
  await userEvent.click(
    screen.getByRole("button", { name: "Confirm early return" }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent("Outcome unknown");
  expect(confirmed).not.toHaveBeenCalled();
  await userEvent.click(
    screen.getByRole("button", { name: "Retry same early return" }),
  );
  expect(await screen.findByRole("status")).toHaveTextContent(
    "Early return recorded",
  );
  const calls = session.client.member.endTravelFlexEarly.mock.calls;
  expect(calls[0]![0]).toEqual(calls[1]![0]);
  expect(calls[0]![0]).toMatchObject({
    memberId: "member-1",
    windowId: "window-1",
    consentVersion: "historic-consent-1",
  });
  expect(confirmed).toHaveBeenCalledTimes(1);
});

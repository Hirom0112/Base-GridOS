import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, test, vi } from "vitest";
import { GetPlanExplanationResponseSchema } from "../api/gen/gridos/v1/api_pb";
import { UnsafeAlternative } from "./unsafe-alternative";

const session = vi.hoisted(() => ({
  identity: { role: "operator" },
  client: { dispatch: { validateUnsafeAlternative: vi.fn() } },
}));
vi.mock("../api/auth", () => ({ useSession: () => session }));

function fixture() {
  return create(GetPlanExplanationResponseSchema, {
    evidence: {
      siteLoadUnits: "kWh",
      deviceSchedules: [
        {
          deviceId: "device-1",
          reserveSelection: 1,
          selectedReserveKwh: 5,
          intervals: [
            {
              beginTime: timestampFromDate(new Date("2026-09-27T12:00:00Z")),
              endTime: timestampFromDate(new Date("2026-09-27T12:05:00Z")),
              setpointKw: 2,
              expectedEnergyKwh: 8,
            },
          ],
        },
      ],
    },
  });
}

beforeEach(() => {
  session.identity.role = "operator";
  session.client.dispatch.validateUnsafeAlternative.mockReset();
});

test("unsafe validation uses a copy of stored schedules without changing the approved candidate", async () => {
  const explanation = fixture();
  session.client.dispatch.validateUnsafeAlternative.mockResolvedValue({
    approved: false,
    violations: [{ code: "POWER_LIMIT" }],
    operatorExplanation: "power limit",
  });
  render(
    <UnsafeAlternative
      eventId="event-1"
      planVersion={3n}
      state={3}
      explanation={explanation}
    />,
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Validate unsafe alternative" }),
  );
  expect(await screen.findByText("Alternative rejected")).toBeVisible();
  expect(screen.getByText("POWER_LIMIT")).toBeVisible();
  expect(
    session.client.dispatch.validateUnsafeAlternative,
  ).toHaveBeenCalledWith(
    expect.objectContaining({
      eventId: "event-1",
      planVersion: 3n,
      alternativePlan: expect.objectContaining({
        eventId: "event-1",
        planVersion: 3n,
        deviceSchedules: [
          expect.objectContaining({
            deviceId: "device-1",
            selectedReserveKwh: 5,
            intervals: [expect.objectContaining({ setpointKw: 1000000 })],
          }),
        ],
      }),
    }),
  );
  expect(
    explanation.evidence!.deviceSchedules[0]!.intervals[0]!.setpointKw,
  ).toBe(2);
});

test("missing schedules and a nonvalidated event cannot submit an alternative", async () => {
  render(
    <UnsafeAlternative
      eventId="event-1"
      planVersion={3n}
      state={4}
      explanation={fixture()}
    />,
  );
  expect(
    screen.getByRole("button", { name: "Validate unsafe alternative" }),
  ).toBeDisabled();
  expect(
    session.client.dispatch.validateUnsafeAlternative,
  ).not.toHaveBeenCalled();
});

test("contradictory validation cannot be displayed as a safe alternative", async () => {
  session.client.dispatch.validateUnsafeAlternative.mockResolvedValue({
    approved: true,
    violations: [{ code: "POWER_LIMIT" }],
    operatorExplanation: "",
  });
  render(
    <UnsafeAlternative
      eventId="event-1"
      planVersion={3n}
      state={3}
      explanation={fixture()}
    />,
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Validate unsafe alternative" }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Invalid safety validation response",
  );
  expect(
    screen.queryByText("Alternative passed validation"),
  ).not.toBeInTheDocument();
});

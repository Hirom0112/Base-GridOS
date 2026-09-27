import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, test, vi } from "vitest";
import { StepUpFailure } from "../api/step-up";
import { EmergencyStopControl } from "./emergency-stop";

const session = vi.hoisted(() => ({
  identity: { mode: "local", role: "operator" },
  client: { events: { emergencyStop: vi.fn() } },
}));
vi.mock("../api/auth", () => ({ useSession: () => session }));

beforeEach(() => {
  session.identity.role = "operator";
  session.client.events.emergencyStop.mockReset();
  HTMLDialogElement.prototype.showModal = function () {
    this.open = true;
  };
});

test("a stop needs confirmation and remains requested after receipt", async () => {
  session.client.events.emergencyStop.mockImplementation(async (request) => ({
    stopRequested: true,
    emergencyStop: { ...request, emergencyStopId: "stop-1" },
  }));
  render(<EmergencyStopControl eventId="event-1" />);
  await userEvent.click(
    screen.getByRole("button", { name: "Request emergency stop" }),
  );
  await userEvent.type(
    screen.getByLabelText("Reason for stopping"),
    "Unexpected response",
  );
  await userEvent.type(screen.getByLabelText("Type STOP to confirm"), "STOP");
  expect(session.client.events.emergencyStop).not.toHaveBeenCalled();
  await userEvent.click(
    screen.getByRole("button", { name: "Confirm stop request" }),
  );
  expect(await screen.findByRole("status")).toHaveTextContent("STOP REQUESTED");
  expect(screen.getByText(/not confirmed stopped/)).toBeVisible();
  expect(session.client.events.emergencyStop).toHaveBeenCalledTimes(1);
});

test("an uncertain response retries the same immutable intent", async () => {
  session.client.events.emergencyStop.mockRejectedValue(
    new Error("Connection interrupted"),
  );
  render(<EmergencyStopControl eventId="event-1" />);
  await userEvent.click(
    screen.getByRole("button", { name: "Request emergency stop" }),
  );
  await userEvent.type(
    screen.getByLabelText("Reason for stopping"),
    "Unexpected response",
  );
  await userEvent.type(screen.getByLabelText("Type STOP to confirm"), "STOP");
  await userEvent.click(
    screen.getByRole("button", { name: "Confirm stop request" }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent("Outcome unknown");
  expect(screen.getByLabelText("Reason for stopping")).toBeDisabled();
  await userEvent.click(
    screen.getByRole("button", { name: "Retry same request" }),
  );
  expect(session.client.events.emergencyStop).toHaveBeenCalledTimes(2);
  expect(session.client.events.emergencyStop.mock.calls[1]?.[0]).toEqual(
    session.client.events.emergencyStop.mock.calls[0]?.[0],
  );
  expect(screen.queryByText("STOP REQUESTED")).not.toBeInTheDocument();
});

test("an analyst cannot request a stop", () => {
  session.identity.role = "analyst";
  render(<EmergencyStopControl eventId="event-1" />);
  expect(screen.queryByRole("button")).not.toBeInTheDocument();
  expect(screen.getByText(/Operator or approver/)).toBeVisible();
});

test("failed step-up authorizes no stop and offers an explicit retry", async () => {
  session.client.events.emergencyStop
    .mockRejectedValueOnce(new StepUpFailure("Step-up authorization denied"))
    .mockImplementationOnce(async (request) => ({
      stopRequested: true,
      emergencyStop: { ...request, emergencyStopId: "stop-authorized" },
    }));
  render(<EmergencyStopControl eventId="event-1" />);
  await userEvent.click(
    screen.getByRole("button", { name: "Request emergency stop" }),
  );
  await userEvent.type(
    screen.getByLabelText("Reason for stopping"),
    "Unexpected response",
  );
  await userEvent.type(screen.getByLabelText("Type STOP to confirm"), "STOP");
  await userEvent.click(
    screen.getByRole("button", { name: "Confirm stop request" }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "No stop command was sent",
  );
  expect(screen.queryByText(/Outcome unknown/)).not.toBeInTheDocument();
  await userEvent.click(
    screen.getByRole("button", { name: "Retry authorization" }),
  );
  expect(await screen.findByRole("status")).toHaveTextContent("STOP REQUESTED");
  expect(session.client.events.emergencyStop.mock.calls[1]?.[0]).toEqual(
    session.client.events.emergencyStop.mock.calls[0]?.[0],
  );
});

test("a later authorization failure preserves an earlier unknown stop outcome", async () => {
  session.client.events.emergencyStop
    .mockRejectedValueOnce(new Error("Connection lost"))
    .mockRejectedValueOnce(new StepUpFailure("Authorization unavailable"));
  render(<EmergencyStopControl eventId="event-1" />);
  await userEvent.click(
    screen.getByRole("button", { name: "Request emergency stop" }),
  );
  await userEvent.type(
    screen.getByLabelText("Reason for stopping"),
    "Unexpected response",
  );
  await userEvent.type(screen.getByLabelText("Type STOP to confirm"), "STOP");
  await userEvent.click(
    screen.getByRole("button", { name: "Confirm stop request" }),
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Retry same request" }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent("Outcome unknown");
  expect(
    screen.queryByText(/No stop command was sent/),
  ).not.toBeInTheDocument();
});

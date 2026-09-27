import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { ListEventCommandsResponseSchema } from "../api/gen/gridos/v1/events_pb";
import { CommandEvidence } from "./commands";

function fixture() {
  const at = timestampFromDate(new Date("2026-09-27T12:00:00Z"));
  return create(ListEventCommandsResponseSchema, {
    commands: [
      {
        intent: {
          commandId: "command-1",
          eventId: "event-1",
          deviceId: "device-1",
          planVersion: 1n,
          generation: 2n,
          setpointKw: 4,
          issuedAt: at,
          effectiveAt: at,
          expiresAt: timestampFromDate(new Date("2026-09-27T12:05:00Z")),
        },
        lifecycleState: 3,
        stateRecordedAt: at,
        receipt: {
          commandId: "command-1",
          acknowledgementId: "receipt-1",
          receiptStatus: 1,
          gatewayId: "gateway-1",
          receivedAt: at,
        },
      },
    ],
  });
}

test("command receipt and expiry remain separate from measured delivery", () => {
  render(<CommandEvidence eventId="event-1" data={fixture()} />);
  const commands = screen.getByRole("region", { name: "Command fan-out" });
  expect(commands).toHaveTextContent("command-1");
  expect(commands).toHaveTextContent("4 kW");
  expect(commands).toHaveTextContent("ACKNOWLEDGED");
  expect(commands).toHaveTextContent("2026-09-27T12:05:00.000Z");
  expect(
    screen.getByRole("region", { name: "Measured interval verification" }),
  ).toHaveTextContent("No measured verification intervals returned");
  expect(
    screen.getByRole("region", { name: "Safe return evidence" }),
  ).toHaveTextContent("No zero-setpoint intents returned");
});

test("a receipt for another command cannot establish acknowledgement", () => {
  const data = fixture();
  data.commands[0]!.receipt!.commandId = "foreign-command";
  render(<CommandEvidence eventId="event-1" data={data} />);
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Command evidence is invalid",
  );
  expect(screen.queryByRole("table")).not.toBeInTheDocument();
});

test("foreign event commands are rejected", () => {
  render(<CommandEvidence eventId="different-event" data={fixture()} />);
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Command evidence is invalid",
  );
});

test("zero setpoints and measured intervals do not claim a stopped fleet", () => {
  const data = fixture();
  data.commands[0]!.intent!.setpointKw = 0;
  data.verificationIntervals = create(ListEventCommandsResponseSchema, {
    verificationIntervals: [
      {
        beginTime: timestampFromDate(new Date("2026-09-27T12:00:00Z")),
        endTime: timestampFromDate(new Date("2026-09-27T12:05:00Z")),
        requestedKw: 4,
        commandedKw: 4,
        deliveredKw: 3.5,
        trackingErrorKw: -0.5,
        confidence: 0.9,
        measurementBoundary: "METER_NET_EXPORT",
        baselineMethod: "DIRECT",
        valueKind: "MEASURED",
      },
    ],
  }).verificationIntervals;
  render(<CommandEvidence eventId="event-1" data={data} />);
  expect(
    screen.getByRole("region", { name: "Safe return evidence" }),
  ).toHaveTextContent("1 zero-setpoint intents");
  expect(
    screen.getByRole("region", { name: "Safe return evidence" }),
  ).toHaveTextContent("do not confirm the fleet has stopped");
  expect(
    screen.getByRole("region", { name: "Measured interval verification" }),
  ).toHaveTextContent("3.5 kW");
  expect(
    screen.getByRole("region", { name: "Measured interval verification" }),
  ).toHaveTextContent("METER_NET_EXPORT");
  expect(screen.queryByText("Fleet stopped")).not.toBeInTheDocument();
});

import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { ReplayEventResponseSchema } from "../api/gen/gridos/v1/api_pb";
import { ReplayEvidence } from "./replay";
import {
  ReplayClockProvider,
  useReplayClock,
  ReplayClock,
} from "./replay-clock";

function ClockProbe() {
  const { position } = useReplayClock();
  return (
    <output aria-label="Clock probe">
      {position
        ? `${position.eventId}:${position.index}:${position.at.seconds}`
        : "current"}
    </output>
  );
}

test("the replay slider publishes one timestamp and returning current clears it", () => {
  const data = create(ReplayEventResponseSchema, {
    seed: 42n,
    inputSnapshotId: "input-1",
    eligibilitySnapshotId: "eligibility-1",
    policyVersion: "policy-1",
    solverVersion: "solver-1",
    codeVersion: "code-1",
    fleetSha256: "a".repeat(64),
    diffStatus: "IDENTICAL",
    updates: [1, 2].map((sequence) => ({
      sequence: BigInt(sequence),
      occurredAt: timestampFromDate(new Date(`2026-09-27T12:00:0${sequence}Z`)),
      actorId: "workflow",
      action: `ACTION_${sequence}`,
      state: sequence,
      reason: "Recorded",
    })),
  });
  render(
    <ReplayClockProvider>
      <ReplayClock />
      <ClockProbe />
      <ReplayEvidence data={data} eventId="event-1" />
    </ReplayClockProvider>,
  );
  expect(screen.getByLabelText("Clock probe")).toHaveTextContent(
    `event-1:0:${data.updates[0]!.occurredAt!.seconds}`,
  );
  fireEvent.change(screen.getByRole("slider"), { target: { value: "1" } });
  expect(screen.getByLabelText("Clock probe")).toHaveTextContent(
    `event-1:1:${data.updates[1]!.occurredAt!.seconds}`,
  );
  expect(screen.getByLabelText("Replay time")).toHaveTextContent("12:00:02");
  fireEvent.click(
    screen.getByRole("button", { name: "Return to current geography" }),
  );
  expect(screen.getByLabelText("Clock probe")).toHaveTextContent("current");
});

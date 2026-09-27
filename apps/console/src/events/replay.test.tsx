import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import {
  ReplayEventResponseSchema,
  EventTimelineEntrySchema,
} from "../api/gen/gridos/v1/api_pb";
import { ReplayEvidence } from "./replay";

function fixture() {
  return create(ReplayEventResponseSchema, {
    seed: 42n,
    inputSnapshotId: "input-1",
    eligibilitySnapshotId: "eligible-1",
    policyVersion: "policy-1",
    solverVersion: "solver-1",
    fallbackVersion: "fallback-1",
    codeVersion: "code-1",
    fleetSha256: "a".repeat(64),
    scenarioSha256: "b".repeat(64),
    diffStatus: "IDENTICAL",
    updates: [1, 2].map((sequence) =>
      create(EventTimelineEntrySchema, {
        sequence: BigInt(sequence),
        occurredAt: timestampFromDate(
          new Date(`2026-09-27T12:00:0${sequence}Z`),
        ),
        actorId: "workflow",
        action: `ACTION_${sequence}`,
        state: sequence,
        reason: `Recorded ${sequence}`,
      }),
    ),
  });
}

test("one replay position controls the visible audit history and timestamp", () => {
  render(<ReplayEvidence data={fixture()} />);
  expect(screen.getByText("IDENTICAL")).toBeVisible();
  expect(screen.getByText("ACTION 1")).toBeVisible();
  expect(screen.queryByText("ACTION 2")).not.toBeInTheDocument();
  fireEvent.change(screen.getByRole("slider", { name: "Replay position" }), {
    target: { value: "1" },
  });
  expect(screen.getByText("ACTION 2")).toBeVisible();
  expect(screen.getByLabelText("Replay time")).toHaveTextContent(
    "2026-09-27T12:00:02.000Z",
  );
  expect(screen.getByText(/input-1/)).toBeVisible();
});

test("contradictory identical status with differences is rejected", () => {
  const data = fixture();
  data.differences = [
    {
      $typeName: "gridos.v1.ReplayDifference",
      field: "plan",
      expectedJson: "{}",
      actualJson: "[]",
    },
  ];
  render(<ReplayEvidence data={data} />);
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Replay evidence is invalid",
  );
});

test("reversed replay update times are rejected", () => {
  const data = fixture();
  data.updates[1]!.occurredAt = timestampFromDate(
    new Date("2026-09-26T12:00:00Z"),
  );
  render(<ReplayEvidence data={data} />);
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Replay evidence is invalid",
  );
  expect(screen.queryByRole("slider")).not.toBeInTheDocument();
});

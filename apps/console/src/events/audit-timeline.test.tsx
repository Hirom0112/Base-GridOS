import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { EventTimelineEntrySchema } from "../api/gen/gridos/v1/api_pb";
import { DispatchEventState } from "../api/gen/gridos/v1/dispatch_pb";
import { AuditRecords } from "./audit-timeline";

const entry = create(EventTimelineEntrySchema, {
  sequence: 1n,
  occurredAt: timestampFromDate(new Date("2026-09-26T12:00:00Z")),
  actorId: "workflow",
  action: "EVENT_STATE_TRANSITIONED",
  previousState: DispatchEventState.APPROVED,
  state: DispatchEventState.COMMANDS_PERSISTED,
  reason: "Approved plan persisted",
});

test("audit records preserve transitions, decisions, actors and sequence", () => {
  render(
    <AuditRecords
      entries={[
        entry,
        create(EventTimelineEntrySchema, {
          ...entry,
          sequence: 2n,
          action: "RETRY_SCHEDULED",
          reason: "Gateway timeout; bounded retry scheduled",
          previousState: DispatchEventState.UNSPECIFIED,
          state: DispatchEventState.UNSPECIFIED,
        }),
      ]}
    />,
  );
  expect(screen.getAllByRole("listitem")).toHaveLength(2);
  expect(screen.getByText("Approved → Commands persisted")).toBeVisible();
  expect(
    screen.getByText("Gateway timeout; bounded retry scheduled"),
  ).toBeVisible();
  expect(screen.getAllByText(/workflow/)).toHaveLength(2);
  expect(screen.getAllByText("2026-09-26T12:00:00.000Z")).toHaveLength(2);
});

test("invalid audit timestamps cannot appear as a valid history", () => {
  render(
    <AuditRecords
      entries={[
        create(EventTimelineEntrySchema, { ...entry, occurredAt: undefined }),
      ]}
    />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Audit evidence is invalid",
  );
  expect(screen.queryByRole("list")).not.toBeInTheDocument();
});

test("duplicate or reversed audit sequences are rejected", () => {
  render(<AuditRecords entries={[entry, entry]} />);
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Audit evidence is invalid",
  );
});

test("an empty audit response is not a completed history", () => {
  render(<AuditRecords entries={[]} />);
  expect(
    screen.getByText("No audit records returned by the server."),
  ).toBeVisible();
});

import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { EventExceptionSchema } from "../api/gen/gridos/v1/events_pb";
import { EventExceptions } from "./exceptions";

const record = create(EventExceptionSchema, {
  kind: 2,
  occurredAt: timestampFromDate(new Date("2026-09-27T12:00:00Z")),
  eventId: "event-1",
  deviceId: "device-1",
  commandId: "command-1",
  evidenceId: "evidence-1",
  detail: "Receipt deadline passed",
});

test("shows uncertainty and recovery as separate server records", () => {
  render(
    <EventExceptions
      eventId="event-1"
      records={[
        record,
        create(EventExceptionSchema, {
          ...record,
          kind: 4,
          detail: "Retry scheduled",
        }),
      ]}
    />,
  );
  expect(screen.getByText("UNCERTAIN_COMMAND")).toBeVisible();
  expect(screen.getByText("COMMAND_RETRY")).toBeVisible();
  expect(screen.getByText("Receipt deadline passed")).toBeVisible();
  expect(screen.getByText("Retry scheduled")).toBeVisible();
  expect(screen.getAllByText(/command-1/)).toHaveLength(2);
  expect(screen.getAllByText(/evidence-1/)).toHaveLength(2);
  expect(screen.getByText(/does not prove recovery/)).toBeVisible();
});

test.each([
  { eventId: "another-event" },
  { occurredAt: undefined },
  { kind: 99 },
])("rejects invalid or unrelated evidence: %j", (change) => {
  render(
    <EventExceptions
      eventId="event-1"
      records={[create(EventExceptionSchema, { ...record, ...change })]}
    />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Exception evidence is invalid",
  );
  expect(screen.queryByRole("list")).not.toBeInTheDocument();
});

test("no exceptions does not claim a healthy fleet", () => {
  render(<EventExceptions eventId="event-1" records={[]} />);
  expect(
    screen.getByText("No exception records returned by the server."),
  ).toBeVisible();
  expect(screen.queryByText(/All devices healthy/)).not.toBeInTheDocument();
});

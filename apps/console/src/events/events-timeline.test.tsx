import { create } from "@bufbuild/protobuf";
import { render, screen, within } from "@testing-library/react";
import { expect, test } from "vitest";
import {
  DispatchEventSchema,
  DispatchEventState,
} from "../api/gen/gridos/v1/dispatch_pb";
import { EventHistory } from "./events-timeline";

const event = create(DispatchEventSchema, {
  eventId: "event",
  planVersion: 1n,
  state: DispatchEventState.SENT,
  createdAt: { seconds: 1786575300n },
  updatedAt: { seconds: 1786575420n },
  launch: {
    requestedBy: "local-approver",
    requestedAt: { seconds: 1786575360n },
    planVersion: 1n,
  },
});

test("timeline exposes only known server records with their timestamps", () => {
  render(<EventHistory event={event} />);
  const rows = within(
    screen.getByRole("list", { name: "Known event records" }),
  ).getAllByRole("listitem");
  expect(rows).toHaveLength(3);
  expect(rows[0]).toHaveTextContent("Event created");
  expect(rows[1]).toHaveTextContent("Launch requested");
  expect(rows[1]).toHaveTextContent("local-approver · plan v1");
  expect(rows[2]).toHaveTextContent("Latest recorded state · Sent");
  expect(rows.every((row) => row.querySelector("time[datetime]"))).toBe(true);
  expect(screen.getByText(/not a complete transition history/)).toBeVisible();
  expect(screen.queryByText("Approved")).not.toBeInTheDocument();
});

test("missing or invalid timestamps never become fabricated milestones", () => {
  const { rerender } = render(<EventHistory event={event} />);
  expect(screen.getByText("Launch requested")).toBeVisible();
  rerender(
    <EventHistory
      event={{
        ...event,
        launch: undefined,
        createdAt: undefined,
        updatedAt: { ...event.updatedAt!, seconds: 253402300800n },
      }}
    />,
  );
  expect(screen.queryByText("Launch requested")).not.toBeInTheDocument();
  expect(screen.queryByRole("listitem")).not.toBeInTheDocument();
  expect(
    screen.getByText("No valid record timestamps returned."),
  ).toBeVisible();
});

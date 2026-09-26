import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import {
  DispatchEventSchema,
  DispatchEventState,
} from "../api/gen/gridos/v1/dispatch_pb";
import { ApprovalActions } from "./approval";

test("approval does not expose launch before the server reports APPROVED", () => {
  const event = create(DispatchEventSchema, {
    eventId: "event",
    state: DispatchEventState.VALIDATED,
    planVersion: 1n,
  });
  render(<ApprovalActions event={event} role="approver" onConfirm={vi.fn()} />);
  expect(screen.getByRole("button", { name: "Review approval" })).toBeEnabled();
  expect(
    screen.queryByRole("button", { name: "Review launch" }),
  ).not.toBeInTheDocument();
});

test("launch is a separate action and remains unavailable to an operator", () => {
  const event = create(DispatchEventSchema, {
    eventId: "event",
    state: DispatchEventState.APPROVED,
    planVersion: 1n,
  });
  const { rerender } = render(
    <ApprovalActions event={event} role="approver" onConfirm={vi.fn()} />,
  );
  expect(screen.getByRole("button", { name: "Review launch" })).toBeEnabled();
  rerender(
    <ApprovalActions event={event} role="operator" onConfirm={vi.fn()} />,
  );
  expect(
    screen.queryByRole("button", { name: "Review launch" }),
  ).not.toBeInTheDocument();
  expect(screen.getByText(/Approver role required/)).toBeVisible();
});

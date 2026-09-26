import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { DispatchForm, dispatchSchema } from "./dispatch-form";

const valid = {
  region: "LZ_AEN",
  begin: "2026-09-26T23:00",
  end: "2026-09-27T01:00",
  targetMw: 5,
  boundary: "METER_NET_EXPORT",
};

test("dispatch-form validates the window, target and measurement boundary", () => {
  expect(dispatchSchema.safeParse(valid).success).toBe(true);
  for (const value of [
    { ...valid, end: valid.begin },
    { ...valid, targetMw: -1 },
    { ...valid, targetMw: Infinity },
    { ...valid, region: "OTHER" },
    { ...valid, boundary: "UNKNOWN" },
  ])
    expect(dispatchSchema.safeParse(value).success).toBe(false);
});

test("dispatch-form shows UTC and submits explicit operator input", async () => {
  const submit = vi.fn().mockResolvedValue(undefined);
  render(<DispatchForm onSubmit={submit} />);
  expect(screen.getByText(/All times are UTC/)).toBeVisible();
  const user = userEvent.setup();
  await user.type(screen.getByLabelText("Target power (MW)"), "5");
  expect(
    screen.getByRole("button", { name: "Create dispatch plan" }),
  ).toBeVisible();
  await user.click(
    screen.getByRole("button", { name: "Create dispatch plan" }),
  );
  expect(submit).not.toHaveBeenCalled();
});

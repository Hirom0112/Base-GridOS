import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import { LocalSession, useSession } from "./auth";

function Identity() {
  const session = useSession();
  return (
    <span>
      {session.identity.role}: {session.identity.mode}
    </span>
  );
}

test("local auth exposes the explicit stub and selected role", async () => {
  render(
    <LocalSession>
      <Identity />
    </LocalSession>,
  );
  expect(screen.getByText("operator: local")).toBeVisible();
  expect(screen.getByText(/STUBBED/)).toBeVisible();
  await userEvent.selectOptions(
    screen.getByRole("combobox", { name: "Demo role" }),
    "approver",
  );
  expect(screen.getByText("approver: local")).toBeVisible();
});

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import { renderToString } from "react-dom/server";
import { LocalSession, useSession } from "./auth";

function Identity() {
  const session = useSession();
  return (
    <span>
      {session.identity.role}: {session.identity.mode}
    </span>
  );
}

test("server-rendered identity controls wait for hydration", () => {
  const markup = document.createElement("div");
  markup.innerHTML = renderToString(
    <LocalSession>
      <Identity />
    </LocalSession>,
  );
  expect(markup.querySelector("select")).toBeDisabled();
  expect(markup.querySelector('input[type="checkbox"]')).toBeDisabled();
  expect(markup).toHaveTextContent("operator: local");
});

test("local auth exposes the pending identity provider and selected role", async () => {
  render(
    <LocalSession>
      <Identity />
    </LocalSession>,
  );
  expect(screen.getByText("operator: local")).toBeVisible();
  expect(screen.getByText("LOCAL IDENTITY · PENDING-LIVE")).toBeVisible();
  await userEvent.selectOptions(
    screen.getByRole("combobox", { name: "Demo role" }),
    "approver",
  );
  expect(screen.getByText("approver: local")).toBeVisible();
});

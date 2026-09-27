import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { SessionProvider } from "../api/auth";
import { MemberHome } from "./home";

test("local demonstration can select its bound household", () => {
  render(
    <SessionProvider
      identity={{
        mode: "local",
        role: "member",
        permissions: [],
        memberId: "member-1",
      }}
    >
      <MemberHome />
    </SessionProvider>,
  );
  expect(screen.getByLabelText("Member ID")).toHaveValue("member-1");
  expect(screen.getByRole("button", { name: "Open household" })).toBeVisible();
});

test("production member sessions do not request internal household IDs", () => {
  render(
    <SessionProvider
      identity={{
        mode: "clerk",
        role: "member",
        userId: "user-1",
        getToken: async () => "token",
      }}
    >
      <MemberHome />
    </SessionProvider>,
  );
  expect(
    screen.getByText(
      "Your household connection is not available for this session.",
    ),
  ).toBeVisible();
  expect(screen.queryByLabelText("Site ID")).not.toBeInTheDocument();
});

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import { Shell } from "./shell";

test.each(["dark", "light"] as const)(
  "shell preserves the %s static frame",
  async (theme) => {
    const { container } = render(<Shell initialTheme={theme} />);
    expect(screen.getByRole("banner")).toHaveTextContent("GridOS");
    expect(screen.getByRole("main")).toHaveAccessibleName("Austin fleet");
    expect(
      screen.getByRole("complementary", { name: "Fleet evidence" }),
    ).toBeVisible();
    expect(screen.getByRole("contentinfo")).toHaveTextContent(
      "Fleet state unavailable",
    );
    expect(screen.getAllByText("SIMULATED").length).toBeGreaterThan(0);
    const loop = screen.getByRole("navigation", { name: "Operating loop" });
    expect(within(loop).getByRole("link", { name: /Observe/ })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(within(loop).getAllByRole("listitem")).toHaveLength(7);
    expect(screen.getByText("Scenario time unavailable")).toBeVisible();
    expect(screen.getByText("No fleet observation received")).toBeVisible();
    expect(container.querySelector("canvas")).toBeNull();
    expect(container).not.toHaveTextContent(
      /FLEET READY|All systems nominal|42\.630/,
    );
    await expect(container.innerHTML).toMatchFileSnapshot(
      `./__snapshots__/shell.${theme}.html`,
    );
  },
);

test("shell supports keyboard access and theme changes without moving focus", async () => {
  const user = userEvent.setup();
  render(<Shell initialTheme="dark" />);
  await user.tab();
  expect(
    screen.getByRole("link", { name: "Skip to fleet overview" }),
  ).toHaveFocus();
  await user.tab();
  const toggle = screen.getByRole("button", { name: "Use light theme" });
  expect(toggle).toHaveFocus();
  await user.keyboard("{Enter}");
  expect(screen.getByRole("button", { name: "Use dark theme" })).toHaveFocus();
  expect(screen.getByRole("main").closest("[data-theme]")).toHaveAttribute(
    "data-theme",
    "light",
  );
});

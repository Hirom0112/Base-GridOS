import { createMemoryHistory, RouterProvider } from "@tanstack/react-router";
import { render, screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { getRouter } from "./router";

test("serves the root route through the application router", async () => {
  vi.stubEnv("VITE_GRIDOS_AUTH_MODE", "local");
  const router = getRouter();
  router.update({
    history: createMemoryHistory({ initialEntries: ["/"] }),
    scrollRestoration: false,
  });
  render(<RouterProvider router={router} />);
  expect(
    await screen.findByRole("heading", { name: "Austin fleet" }),
  ).toBeVisible();
  expect(router.state.location.pathname).toBe("/fleet");
  vi.unstubAllEnvs();
});

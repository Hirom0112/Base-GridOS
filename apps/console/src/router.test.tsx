import { createMemoryHistory, RouterProvider } from "@tanstack/react-router";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { getRouter } from "./router";

test("serves the root route through the application router", async () => {
  const router = getRouter();
  router.update({ history: createMemoryHistory({ initialEntries: ["/"] }) });
  render(<RouterProvider router={router} />);
  expect(await screen.findByRole("heading", { name: "GridOS" })).toBeVisible();
});

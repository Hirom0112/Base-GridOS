import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/_console/events/$eventId")({
  component: Outlet,
});

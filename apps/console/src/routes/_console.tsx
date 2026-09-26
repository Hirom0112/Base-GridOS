import { createFileRoute, Outlet } from "@tanstack/react-router";
import { AuthBoundary } from "../api/auth";
import { Console } from "../console";

export const Route = createFileRoute("/_console")({ component: ConsoleRoute });

function ConsoleRoute() {
  return (
    <AuthBoundary>
      <Console>
        <Outlet />
      </Console>
    </AuthBoundary>
  );
}

import { createFileRoute, Outlet } from "@tanstack/react-router";
import { AuthBoundary } from "../api/auth";
import { Console } from "../console";
import { ReplayClockProvider } from "../events/replay-clock";

export const Route = createFileRoute("/_console")({ component: ConsoleRoute });

function ConsoleRoute() {
  return (
    <AuthBoundary>
      <ReplayClockProvider>
        <Console>
          <Outlet />
        </Console>
      </ReplayClockProvider>
    </AuthBoundary>
  );
}

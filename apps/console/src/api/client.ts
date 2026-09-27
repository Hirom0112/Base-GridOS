import { createClient, ConnectError, Code } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { QueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { stepUpAuthorization } from "./step-up";
import {
  ContextService,
  ReportService,
  ReplayService,
  DispatchService,
  FleetService,
  EventsService,
} from "./gen/gridos/v1/api_pb";

export const roleSchema = z.enum([
  "operator",
  "approver",
  "analyst",
  "partner",
  "service",
  "member",
]);
export type Role = z.infer<typeof roleSchema>;
export type Identity =
  | { mode: "local"; role: Role; permissions: "site_location"[] }
  | {
      mode: "clerk";
      role: Role;
      userId: string;
      getToken: () => Promise<string | null>;
    };

export function createConsoleClient(
  baseUrl: string,
  identity: Identity,
  fetcher: typeof fetch = fetch,
) {
  const transport = createConnectTransport({
    baseUrl,
    fetch: fetcher,
    defaultTimeoutMs: 15000,
    interceptors: [
      (next) => async (request) => {
        if (identity.mode === "local") {
          request.header.set("X-GridOS-Role", identity.role);
          if (identity.permissions.length)
            request.header.set(
              "X-GridOS-Permissions",
              identity.permissions.join(","),
            );
        } else {
          const token = await identity.getToken();
          if (!token)
            throw new ConnectError("Sign in to continue", Code.Unauthenticated);
          request.header.set("Authorization", `Bearer ${token}`);
        }
        return next(request);
      },
      stepUpAuthorization(identity, fetcher),
    ],
  });
  return {
    replay: createClient(ReplayService, transport),
    reports: createClient(ReportService, transport),
    context: createClient(ContextService, transport),
    fleet: createClient(FleetService, transport),
    dispatch: createClient(DispatchService, transport),
    events: createClient(EventsService, transport),
  };
}

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 5000,
        retry: (count, error) =>
          count < 1 &&
          error instanceof ConnectError &&
          [Code.Unavailable, Code.DeadlineExceeded].includes(error.code),
        refetchOnWindowFocus: false,
      },
      mutations: { retry: false },
    },
  });
}

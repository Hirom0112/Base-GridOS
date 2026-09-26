import {
  createContext,
  lazy,
  Suspense,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { useHydrated } from "@tanstack/react-router";
import {
  createConsoleClient,
  createQueryClient,
  roleSchema,
  type Identity,
  type Role,
} from "./client";

const ClerkSession = lazy(() => import("./clerk"));
const SessionContext = createContext<{
  identity: Identity;
  client: ReturnType<typeof createConsoleClient>;
} | null>(null);

export function useSession() {
  const session = useContext(SessionContext);
  if (!session) throw new Error("An authenticated session is required");
  return session;
}

export function SessionProvider({
  identity,
  children,
}: {
  identity: Identity;
  children: ReactNode;
}) {
  const [queries] = useState(createQueryClient);
  const session = useMemo(
    () => ({ identity, client: createConsoleClient("/rpc", identity) }),
    [identity],
  );
  return (
    <SessionContext.Provider value={session}>
      <QueryClientProvider client={queries}>{children}</QueryClientProvider>
    </SessionContext.Provider>
  );
}

export function LocalSession({ children }: { children: ReactNode }) {
  const hydrated = useHydrated();
  const [role, setRole] = useState<Role>("operator");
  const [permissions, setPermissions] = useState<"site_location"[]>([]);
  const identity = useMemo<Identity>(
    () => ({ mode: "local", role, permissions }),
    [role, permissions],
  );
  return (
    <>
      <div className="local-session">
        <span>LOCAL DEMO · STUBBED IDENTITY</span>
        <label>
          Demo role
          <select
            aria-label="Demo role"
            disabled={!hydrated}
            value={role}
            onChange={(event) => setRole(roleSchema.parse(event.target.value))}
          >
            {roleSchema.options.map((value) => (
              <option key={value}>{value}</option>
            ))}
          </select>
        </label>
        <label>
          <input
            type="checkbox"
            disabled={!hydrated}
            checked={permissions.length > 0}
            onChange={(event) =>
              setPermissions(event.target.checked ? ["site_location"] : [])
            }
          />
          Exact-site permission
        </label>
      </div>
      <SessionProvider
        key={`${role}:${permissions.join()}`}
        identity={identity}
      >
        {children}
      </SessionProvider>
    </>
  );
}

export function AuthBoundary({ children }: { children: ReactNode }) {
  if (import.meta.env.VITE_GRIDOS_AUTH_MODE === "local")
    return <LocalSession>{children}</LocalSession>;
  if (!import.meta.env.VITE_CLERK_PUBLISHABLE_KEY)
    return (
      <main className="auth-gate">
        <h1>Sign-in is not configured</h1>
        <p>
          Configure Clerk or start the local demonstration with
          GRIDOS_AUTH_MODE=local.
        </p>
      </main>
    );
  return (
    <Suspense fallback={<main className="auth-gate">Loading sign-in…</main>}>
      <ClerkSession>{children}</ClerkSession>
    </Suspense>
  );
}

import {
  ClerkProvider,
  SignIn,
  useAuth,
  useUser,
} from "@clerk/tanstack-react-start";
import { useMemo, type ReactNode } from "react";
import { SessionProvider } from "./auth";
import { roleSchema, type Identity } from "./client";

export default function ClerkSession({ children }: { children: ReactNode }) {
  return (
    <ClerkProvider publishableKey={import.meta.env.VITE_CLERK_PUBLISHABLE_KEY}>
      <SignedSession>{children}</SignedSession>
    </ClerkProvider>
  );
}

function SignedSession({ children }: { children: ReactNode }) {
  const { isLoaded, isSignedIn, getToken } = useAuth();
  const { user } = useUser();
  const role = roleSchema.safeParse(user?.publicMetadata.role);
  const identity = useMemo<Identity | null>(
    () =>
      role.success && user?.id
        ? { mode: "clerk", role: role.data, userId: user.id, getToken }
        : null,
    [role.success, role.data, user?.id, getToken],
  );
  if (!isLoaded) return <main className="auth-gate">Loading session…</main>;
  if (!isSignedIn)
    return (
      <main className="auth-gate">
        <SignIn routing="hash" />
      </main>
    );
  if (!identity)
    return (
      <main className="auth-gate">
        <h1>Access not assigned</h1>
        <p>Your account needs an authorized GridOS role.</p>
      </main>
    );
  return <SessionProvider identity={identity}>{children}</SessionProvider>;
}

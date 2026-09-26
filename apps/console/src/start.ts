import { clerkMiddleware } from "@clerk/tanstack-react-start/server";
import { createStart } from "@tanstack/react-start";

export const startInstance = createStart(() => ({
  requestMiddleware:
    process.env.GRIDOS_AUTH_MODE !== "local" && process.env.CLERK_SECRET_KEY
      ? [clerkMiddleware()]
      : [],
}));

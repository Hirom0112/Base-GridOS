import { useEffect } from "react";
import { z } from "zod";
import type { Role } from "./api/client";

const stageSchema = z.enum(["observe", "plan", "execute", "report", "compare"]);
const enabled = z.enum(["true", "false"]).default("false");
const projectKey = z.string().regex(/^phc_[a-zA-Z0-9]+$/);
const secureUrl = z
  .url()
  .refine((value) => new URL(value).protocol === "https:");
const environment = z.object({
  VITE_GRIDOS_SENTRY_ENABLED: enabled,
  VITE_GRIDOS_SENTRY_DSN: z.string().optional(),
  VITE_GRIDOS_POSTHOG_ENABLED: enabled,
  VITE_GRIDOS_POSTHOG_KEY: z.string().optional(),
  VITE_GRIDOS_POSTHOG_HOST: z.string().optional(),
});

export function observabilityConfig(input: unknown) {
  const env = environment.parse(input);
  return {
    sentry:
      env.VITE_GRIDOS_SENTRY_ENABLED === "true"
        ? secureUrl
            .refine((value) => new URL(value).username.length > 0)
            .parse(env.VITE_GRIDOS_SENTRY_DSN)
        : null,
    posthog:
      env.VITE_GRIDOS_POSTHOG_ENABLED === "true"
        ? {
            key: projectKey.parse(env.VITE_GRIDOS_POSTHOG_KEY),
            host: secureUrl
              .refine((value) => {
                const url = new URL(value);
                return (
                  !url.username &&
                  !url.password &&
                  !url.search &&
                  !url.hash &&
                  url.pathname === "/"
                );
              })
              .parse(env.VITE_GRIDOS_POSTHOG_HOST),
          }
        : null,
  };
}

export function operatorStage(pathname: string, role: Role) {
  if (!["operator", "approver", "analyst", "service"].includes(role))
    return null;
  if (["/fleet", "/map"].includes(pathname)) return "observe";
  if (/^\/dispatch\/[^/]+$/.test(pathname)) return "plan";
  if (pathname === "/events/compare") return "compare";
  if (/^\/events\/[^/]+\/report$/.test(pathname)) return "report";
  if (/^\/events\/[^/]+\/?$/.test(pathname)) return "execute";
  return null;
}

export function posthogPayload(
  stage: unknown,
  apiKey: string,
  sessionId: string,
  eventId: string,
) {
  return {
    api_key: projectKey.parse(apiKey),
    event: "operator_view",
    uuid: z.uuid().parse(eventId),
    distinct_id: z.uuid().parse(sessionId),
    timestamp: new Date().toISOString(),
    properties: {
      stage: stageSchema.parse(stage),
      $process_person_profile: false,
    },
  };
}

export function sentryPayload(event: unknown, stage: unknown) {
  const parsed = z
    .object({
      event_id: z
        .string()
        .regex(/^[a-f0-9]{32}$/)
        .optional(),
      timestamp: z.number().finite().optional(),
    })
    .parse(event);
  return {
    ...parsed,
    level: "error" as const,
    message: "Console runtime error",
    tags: { stage: stageSchema.parse(stage) },
  };
}

export function useObservability(pathname: string, role: Role) {
  const stage = operatorStage(pathname, role);
  useEffect(() => {
    if (!stage) return;
    let config: ReturnType<typeof observabilityConfig>;
    try {
      config = observabilityConfig(import.meta.env);
    } catch {
      console.warn("Optional console telemetry configuration is invalid.");
      return;
    }
    if (!config.sentry && !config.posthog) return;
    const abort = new AbortController();
    let stop: (() => void) | undefined;
    if (config.posthog) {
      const payload = posthogPayload(
        stage,
        config.posthog.key,
        crypto.randomUUID(),
        crypto.randomUUID(),
      );
      void fetch(new URL("/i/v0/e/", config.posthog.host), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
        credentials: "omit",
        referrerPolicy: "no-referrer",
        signal: AbortSignal.any([abort.signal, AbortSignal.timeout(5000)]),
      })
        .then((response) => {
          if (!response.ok) throw new Error("Analytics unavailable");
        })
        .catch(() => {
          if (!abort.signal.aborted)
            console.warn("Optional console analytics is unavailable.");
        });
    }
    if (config.sentry) {
      const dsn = config.sentry;
      void import("./error-reporting")
        .then(({ startErrorReporting }) => {
          if (!abort.signal.aborted) stop = startErrorReporting(dsn, stage);
        })
        .catch(() => {
          if (!abort.signal.aborted)
            console.warn("Optional console error reporting is unavailable.");
        });
    }
    return () => {
      abort.abort();
      stop?.();
    };
  }, [stage]);
}

import { afterEach, expect, test, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import {
  observabilityConfig,
  operatorStage,
  posthogPayload,
  sentryPayload,
  useObservability,
} from "./observability";

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
});

test("only explicit operator analytics send the allowlisted payload", async () => {
  const fetcher = vi.fn().mockResolvedValue(new Response("{}"));
  vi.stubGlobal("fetch", fetcher);
  vi.stubEnv("VITE_GRIDOS_POSTHOG_ENABLED", "false");
  vi.stubEnv("VITE_GRIDOS_SENTRY_ENABLED", "false");
  const disabled = renderHook(() => useObservability("/fleet", "operator"));
  expect(fetcher).not.toHaveBeenCalled();
  disabled.unmount();
  vi.stubEnv("VITE_GRIDOS_POSTHOG_ENABLED", "true");
  vi.stubEnv("VITE_GRIDOS_POSTHOG_KEY", "phc_test");
  vi.stubEnv("VITE_GRIDOS_POSTHOG_HOST", "https://us.i.posthog.com");
  const operator = renderHook(() =>
    useObservability("/events/private-event/report", "operator"),
  );
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1));
  const options: RequestInit = fetcher.mock.calls[0]![1];
  expect(options.credentials).toBe("omit");
  expect(options.referrerPolicy).toBe("no-referrer");
  const body = JSON.parse(String(options.body));
  expect(body.properties).toEqual({
    stage: "report",
    $process_person_profile: false,
  });
  expect(String(options.body)).not.toContain("private-event");
  operator.unmount();
  expect(options.signal?.aborted).toBe(true);
  renderHook(() => useObservability("/member", "member"));
  expect(fetcher).toHaveBeenCalledTimes(1);
});

test("optional telemetry is disabled without explicit flags and validates enabled destinations", () => {
  expect(observabilityConfig({})).toEqual({ sentry: null, posthog: null });
  expect(() =>
    observabilityConfig({ VITE_GRIDOS_POSTHOG_ENABLED: "true" }),
  ).toThrow();
  expect(() =>
    observabilityConfig({
      VITE_GRIDOS_SENTRY_ENABLED: "true",
      VITE_GRIDOS_SENTRY_DSN: "http://untrusted.test/1",
    }),
  ).toThrow();
});

test("operator analytics have no household identity, site identifier, URL or travel state", () => {
  expect(operatorStage("/member", "operator")).toBeNull();
  expect(operatorStage("/fleet", "member")).toBeNull();
  const stage = operatorStage("/events/private-event-id/report", "operator");
  expect(stage).toBe("report");
  const payload = posthogPayload(
    stage,
    "phc_test",
    "e8689b86-132a-4625-ad9b-ce0f1fb276f6",
    "ce423bf1-b459-4544-ac8a-9b4a4bf2c944",
  );
  expect(payload.properties).toEqual({
    stage: "report",
    $process_person_profile: false,
  });
  expect(payload.distinct_id).toBe("e8689b86-132a-4625-ad9b-ce0f1fb276f6");
  expect(JSON.stringify(payload)).not.toContain("private-event-id");
  expect(() =>
    posthogPayload("travel-active", "phc_test", "household-1", "site-1"),
  ).toThrow();
});

test("error payloads discard raw errors, URLs, breadcrumbs and household context", () => {
  const event = sentryPayload(
    {
      event_id: "a".repeat(32),
      timestamp: 123,
      message: "household-1 travel-active",
      user: { id: "household-1" },
      request: { url: "/member/site-1" },
      breadcrumbs: [{ message: "travel-active" }],
      exception: { values: [{ type: "TypeError", value: "site-1" }] },
      extra: { memberId: "household-1" },
      contexts: { travel: true },
    },
    "plan",
  );
  expect(event).toEqual({
    event_id: "a".repeat(32),
    timestamp: 123,
    level: "error",
    message: "Console runtime error",
    tags: { stage: "plan" },
  });
  expect(JSON.stringify(event)).not.toMatch(/household|site-1|travel/);
});

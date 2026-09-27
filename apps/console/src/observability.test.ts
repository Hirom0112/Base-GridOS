import { expect, test } from "vitest";
import {
  observabilityConfig,
  operatorStage,
  posthogPayload,
  sentryPayload,
} from "./observability";

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
    distinct_id: "e8689b86-132a-4625-ad9b-ce0f1fb276f6",
    $process_person_profile: false,
  });
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

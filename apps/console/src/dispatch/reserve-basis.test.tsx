import { create } from "@bufbuild/protobuf";
import { render, screen, within } from "@testing-library/react";
import { expect, test } from "vitest";
import { GetPlanExplanationResponseSchema } from "../api/gen/gridos/v1/api_pb";
import { ExplanationEvidence } from "./explanation";

function explanation() {
  return create(GetPlanExplanationResponseSchema, {
    evidence: {
      reserveBases: [
        {
          deviceId: "device-one",
          hardwareFloorKwh: 2,
          planReserveKwh: 5,
          policyFloorKwh: 5,
          overrideFloorKwh: 8,
          overrideReason: 1,
          overrideSourceId: "weather-alert-one",
          overridePolicyVersion: "storm-v2",
          policyVersion: "balanced-v1",
          effectiveReserveKwh: 8,
          provenance: 5,
          issuedAt: { seconds: 1790503200n },
        },
      ],
      travelFlexBindings: [
        {
          windowId: "window-one",
          memberId: "member-one",
          siteId: "site-one",
          startTime: { seconds: 1790503200n },
          endTime: { seconds: 1790506800n },
          creditType: 2,
          creditCents: 500n,
          consentVersion: "consent-original",
          policyVersion: "travel-original",
          provenance: 5,
          issuedAt: { seconds: 1790503000n },
        },
      ],
    },
  });
}

test("review shows frozen reserve floors and the original consented credit binding", () => {
  render(<ExplanationEvidence explanation={explanation()} />);
  const reserve = screen.getByRole("region", {
    name: "Household reserve basis",
  });
  expect(reserve).toHaveTextContent("device-one");
  expect(reserve).toHaveTextContent("Hardware floor2.000 kWh");
  expect(reserve).toHaveTextContent("Plan reserve5.000 kWh");
  expect(reserve).toHaveTextContent("Effective reserve8.000 kWh");
  expect(reserve).toHaveTextContent("WEATHER");
  expect(reserve).toHaveTextContent("weather-alert-one");
  expect(reserve).toHaveTextContent("storm-v2");
  expect(reserve).toHaveTextContent("SIMULATED");
  const travel = screen.getByRole("region", {
    name: "Travel Flex eligibility",
  });
  expect(travel).toHaveTextContent("window-one");
  expect(travel).toHaveTextContent("500 cents · Fixed event credit");
  expect(travel).toHaveTextContent("consent-original");
  expect(travel).toHaveTextContent("travel-original");
  expect(travel).toHaveTextContent("2026-09-27T10:00:00.000Z");
});

test.each(["source", "reason", "floor", "time"])(
  "invalid override %s is withheld with a live positive control",
  (field) => {
    const input = explanation();
    const view = render(<ExplanationEvidence explanation={input} />);
    expect(
      screen.getByRole("region", { name: "Household reserve basis" }),
    ).toHaveTextContent("weather-alert-one");
    const basis = input.evidence!.reserveBases[0]!;
    if (field === "source") basis.overrideSourceId = "";
    if (field === "reason") basis.overrideReason = 0;
    if (field === "floor") basis.hardwareFloorKwh = NaN;
    if (field === "time") basis.issuedAt = undefined;
    view.rerender(<ExplanationEvidence explanation={input} />);
    const region = screen.getByRole("region", {
      name: "Household reserve basis",
    });
    expect(within(region).getByRole("alert")).toHaveTextContent("invalid");
    expect(region).not.toHaveTextContent("weather-alert-one");
  },
);

test("absent override differs from an explicit zero floor", () => {
  const input = explanation();
  const basis = input.evidence!.reserveBases[0]!;
  basis.overrideFloorKwh = undefined;
  basis.overrideReason = 0;
  basis.overrideSourceId = "";
  basis.overridePolicyVersion = "";
  render(<ExplanationEvidence explanation={input} />);
  expect(
    screen.getByRole("region", { name: "Household reserve basis" }),
  ).toHaveTextContent("No active override in the frozen evidence");
});

test.each(["consent", "credit", "window"])(
  "invalid Travel Flex %s never becomes a bound credit",
  (field) => {
    const input = explanation();
    const view = render(<ExplanationEvidence explanation={input} />);
    expect(
      screen.getByRole("region", { name: "Travel Flex eligibility" }),
    ).toHaveTextContent("500 cents");
    const binding = input.evidence!.travelFlexBindings[0]!;
    if (field === "consent") binding.consentVersion = "";
    if (field === "credit") binding.creditCents = -1n;
    if (field === "window") binding.endTime = binding.startTime;
    view.rerender(<ExplanationEvidence explanation={input} />);
    const region = screen.getByRole("region", {
      name: "Travel Flex eligibility",
    });
    expect(within(region).getByRole("alert")).toHaveTextContent("invalid");
    expect(region).not.toHaveTextContent("500 cents");
  },
);

test("older plans keep absent frozen policy evidence explicit", () => {
  render(
    <ExplanationEvidence
      explanation={create(GetPlanExplanationResponseSchema)}
    />,
  );
  expect(screen.getByText("Frozen reserve basis unavailable.")).toBeVisible();
  expect(
    screen.getByText("Frozen Travel Flex bindings unavailable."),
  ).toBeVisible();
});

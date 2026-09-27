import { readFileSync } from "node:fs";
import { fromJsonString } from "@bufbuild/protobuf";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { expect, test } from "vitest";
import { GetPlanExplanationResponseSchema } from "../api/gen/gridos/v1/api_pb";
import { ExplanationEvidence } from "./explanation";

const recorded = readFileSync(
  "../../testdata/fixtures/api/DispatchService/GetPlanExplanation.json",
  "utf8",
);
const fixture = () =>
  fromJsonString(GetPlanExplanationResponseSchema, recorded);

test("explanation presents recorded reserve, constraints, and interval feasibility", () => {
  render(<ExplanationEvidence explanation={fixture()} />);
  expect(
    screen.getByRole("region", { name: "Optimization explanation" }),
  ).toHaveTextContent("51,948.152 kWh");
  expect(
    screen.getByRole("region", { name: "Constraint margins" }),
  ).toHaveTextContent("RESERVE");
  expect(
    screen.getByRole("region", { name: "Constraint margins" }),
  ).toHaveTextContent("21.628768236948616");
  expect(
    screen.getByRole("table", { name: "Interval feasibility" }),
  ).toHaveTextContent("1.000");
  expect(
    screen.getAllByText("2026-09-27T07:28:00.000Z").length,
  ).toBeGreaterThan(0);
  expect(screen.getByText(/Modeled objective/)).toBeVisible();
  expect(
    screen.getByRole("table", { name: "Economic margin terms" }),
  ).toBeVisible();
});

test.each([NaN, Infinity, -1])(
  "invalid reserve %s never appears as valid plan evidence",
  (reserve) => {
    const explanation = fixture();
    explanation.reserveHeldBackKwh = reserve;
    render(<ExplanationEvidence explanation={explanation} />);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Plan explanation is invalid",
    );
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  },
);

test("missing objective and empty constraints remain explicit gaps", () => {
  const explanation = fixture();
  explanation.objectiveBreakdown = undefined;
  explanation.constraintMargins = [];
  explanation.shortfalls = [];
  render(<ExplanationEvidence explanation={explanation} />);
  expect(screen.getByText("Objective breakdown unavailable.")).toBeVisible();
  expect(screen.getByText("No constraint margins returned.")).toBeVisible();
  expect(screen.getByText("No interval feasibility returned.")).toBeVisible();
});

test("unavailable economic terms remain unknown instead of displaying zero", () => {
  const explanation = fixture();
  explanation.marginExplanation = fromJsonString(
    GetPlanExplanationResponseSchema,
    JSON.stringify({
      marginExplanation: {
        conservativeMargin: 25,
        marginHurdle: 10,
        terms: [
          { name: "Member reward", unavailable: true, source: "Not supplied" },
          { name: "Charging cost", low: 4, high: 8, source: "catalog-v1" },
        ],
      },
    }),
  ).marginExplanation;
  render(<ExplanationEvidence explanation={explanation} />);
  const terms = screen.getByRole("table", { name: "Economic margin terms" });
  expect(
    within(terms).getByRole("row", { name: /Member reward/ }),
  ).toHaveTextContent("Unavailable");
  expect(
    within(terms).getByRole("row", { name: /Charging cost/ }),
  ).toHaveTextContent("4.000–8.000");
});

test("frozen site forecasts retain modeled bounds, lineage, and fallback reason", () => {
  const explanation = fixture();
  explanation.evidence = fromJsonString(
    GetPlanExplanationResponseSchema,
    JSON.stringify({
      evidence: {
        siteLoadUnits: "kWh",
        fallbackUsed: true,
        fallbackReason: "solver timeout",
        siteLoads: [
          {
            siteId: "site-one",
            intervalBeginTime: "2026-09-27T12:00:00Z",
            loadKwh: {
              value: 2,
              lower: 1,
              upper: 3,
              valueKind: "modeled_estimate",
              provenance: "DATA_PROVENANCE_SIMULATED",
              issuedAt: "2026-09-27T11:00:00Z",
              modelVersion: "load-v1",
              featureVersion: "features-v1",
            },
          },
        ],
      },
    }),
  ).evidence;
  render(<ExplanationEvidence explanation={explanation} />);
  const forecasts = screen.getByRole("region", { name: "Forecast intervals" });
  expect(forecasts).toHaveTextContent("2 kWh");
  expect(forecasts).toHaveTextContent("1–3 kWh");
  expect(
    screen.getByRole("img", { name: "Modeled site load and uncertainty" }),
  ).toHaveTextContent("2 kWh; modeled range 1–3 kWh");
  expect(forecasts).toHaveTextContent("MODELED");
  expect(forecasts).toHaveTextContent("SIMULATED");
  expect(forecasts).toHaveTextContent("load-v1");
  expect(forecasts).toHaveTextContent("2026-09-27T11:00:00.000Z");
  expect(
    screen.getByRole("region", { name: "Solver fallback" }),
  ).toHaveTextContent("solver timeout");
});

test("missing frozen evidence does not imply fallback was unused", () => {
  const explanation = fixture();
  explanation.evidence = undefined;
  render(<ExplanationEvidence explanation={explanation} />);
  expect(
    screen.getByRole("region", { name: "Forecast intervals" }),
  ).toHaveTextContent("Frozen forecast evidence unavailable");
  expect(
    screen.getByRole("region", { name: "Solver fallback" }),
  ).toHaveTextContent("Fallback evidence unavailable");
});

test("forecast site search bounds the selector and never shows a mismatched site", () => {
  const explanation = fixture();
  const target = explanation.evidence!.siteLoads[100]!.siteId;
  render(<ExplanationEvidence explanation={explanation} />);
  const search = screen.getByLabelText("Find forecast site");
  expect(screen.getAllByRole("option").length).toBeLessThanOrEqual(50);
  fireEvent.change(search, { target: { value: target } });
  expect(screen.getByLabelText("Forecast site")).toHaveValue(target);
  expect(
    screen.getByRole("table", { name: "Frozen site forecasts" }),
  ).toBeVisible();
  fireEvent.change(search, { target: { value: "not-a-recorded-site" } });
  expect(screen.getByText("No matching forecast sites.")).toBeVisible();
  expect(
    screen.queryByRole("table", { name: "Frozen site forecasts" }),
  ).not.toBeInTheDocument();
});

test("frozen regional forecasts preserve price signs, probabilities, and missing sources", () => {
  const explanation = fixture();
  const value = {
    value: -5,
    lower: -10,
    upper: 0,
    valueKind: "confirmed_public_forward",
    provenance: "DATA_PROVENANCE_CONFIRMED_PUBLIC",
    issuedAt: "2026-09-27T11:00:00Z",
    modelVersion: "price-v1",
    featureVersion: "prices-v1",
  };
  explanation.evidence = fromJsonString(
    GetPlanExplanationResponseSchema,
    JSON.stringify({
      evidence: {
        siteLoadUnits: "kWh",
        regionalPrices: [
          {
            loadZone: "LZ_AEN",
            intervalBeginTime: "2026-09-27T12:00:00Z",
            pricePerMwh: value,
          },
        ],
        outageRisks: [
          {
            county: "Travis",
            intervalBeginTime: "2026-09-27T12:00:00Z",
            probability: {
              ...value,
              value: 0.2,
              lower: 0.1,
              upper: 0.3,
              valueKind: "modeled_estimate",
            },
          },
        ],
        deviceAvailability: [
          {
            deviceId: "device-1",
            intervalBeginTime: "2026-09-27T12:00:00Z",
            probability: {
              ...value,
              value: 0.9,
              lower: 0.8,
              upper: 1,
              valueKind: "modeled_estimate",
            },
          },
        ],
        unavailableSources: ["regional_price:LZ_OTHER"],
      },
    }),
  ).evidence;
  render(<ExplanationEvidence explanation={explanation} />);
  expect(
    screen.getByRole("region", { name: "Frozen regional prices" }),
  ).toHaveTextContent("-5 USD/MWh");
  expect(
    screen.getByRole("region", { name: "Frozen regional prices" }),
  ).toHaveTextContent("confirmed_public_forward");
  expect(
    screen.getByRole("region", { name: "Frozen outage risk" }),
  ).toHaveTextContent("20%");
  expect(
    screen.getByRole("region", { name: "Frozen device availability" }),
  ).toHaveTextContent("90%");
  expect(
    screen.getByRole("region", { name: "Unavailable forecast sources" }),
  ).toHaveTextContent("regional_price:LZ_OTHER");
});

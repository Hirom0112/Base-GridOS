import { readFileSync } from "node:fs";
import { fromJsonString } from "@bufbuild/protobuf";
import { render, screen, within } from "@testing-library/react";
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

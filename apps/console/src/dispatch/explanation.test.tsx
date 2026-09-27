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
  ).toHaveTextContent("50,518.410 kWh");
  expect(
    screen.getByRole("region", { name: "Constraint margins" }),
  ).toHaveTextContent("RESERVE");
  expect(
    screen.getByRole("region", { name: "Constraint margins" }),
  ).toHaveTextContent("-1.7763568394002505e-15");
  expect(
    screen.getByRole("table", { name: "Interval feasibility" }),
  ).toHaveTextContent("1,000.000");
  expect(screen.getByText("2026-09-27T00:16:27.000Z")).toBeVisible();
  expect(screen.getByText(/Modeled objective/)).toBeVisible();
  expect(
    screen.getByText(/Economic margin evidence unavailable/),
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

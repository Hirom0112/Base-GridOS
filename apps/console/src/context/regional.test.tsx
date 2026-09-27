import { readFileSync } from "node:fs";
import { fromJsonString } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import {
  GetWeatherContextResponseSchema,
  GetOutageRiskResponseSchema,
} from "../api/gen/gridos/v1/api_pb";
import { WeatherEvidence, OutageEvidence } from "./regional";

function weather() {
  return fromJsonString(
    GetWeatherContextResponseSchema,
    readFileSync(
      "../../testdata/fixtures/api/ContextService/GetWeatherContext.json",
      "utf8",
    ),
  );
}
function outages() {
  return fromJsonString(
    GetOutageRiskResponseSchema,
    readFileSync(
      "../../testdata/fixtures/api/ContextService/GetOutageRisk.json",
      "utf8",
    ),
  );
}

test("Austin forecasts preserve the interval, source and observation age", () => {
  render(<WeatherEvidence data={weather()} />);
  expect(
    screen.getByRole("table", { name: "Austin weather forecasts" }),
  ).toHaveTextContent("97 °F");
  expect(screen.getAllByText(/CONFIRMED_PUBLIC/).length).toBeGreaterThan(0);
  expect(
    screen.getAllByText(/2026-09-26T00:10:31.000Z/).length,
  ).toBeGreaterThan(0);
  expect(
    screen.getAllByText(/89894\.935393s old at observation/).length,
  ).toBeGreaterThan(0);
  expect(
    screen.getByText("No weather alerts returned by the source."),
  ).toBeVisible();
});

test("Travis historical outage rates are not current probabilities", () => {
  render(<OutageEvidence data={outages()} />);
  expect(screen.getByText("2023-01")).toBeVisible();
  expect(screen.getByText("3.766% historical rate")).toBeVisible();
  expect(screen.getByText(/not a current outage probability/)).toBeVisible();
  expect(screen.getByText(/DERIVED/)).toBeVisible();
});

test("rejects forecasts from a different city", () => {
  const data = weather();
  data.forecasts[0]!.city = "houston";
  render(<WeatherEvidence data={data} />);
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Weather evidence is invalid",
  );
  expect(screen.queryByRole("table")).not.toBeInTheDocument();
});

test("rejects missing source evidence", () => {
  const data = outages();
  data.rates[0]!.source = undefined;
  render(<OutageEvidence data={data} />);
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Outage history is invalid",
  );
  expect(screen.queryByText("3.766% historical rate")).not.toBeInTheDocument();
});

import { readFileSync } from "node:fs";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { ReportEvidence } from "./report";

function fixture(variant = "") {
  const envelope: unknown = JSON.parse(
    readFileSync(
      `../../testdata/fixtures/api/ReportService/GetEventReport${variant}.json`,
      "utf8",
    ),
  );
  if (
    !envelope ||
    typeof envelope !== "object" ||
    !("reportJson" in envelope) ||
    typeof envelope.reportJson !== "string"
  )
    throw new Error("Invalid report fixture");
  return envelope.reportJson;
}

test("report preserves modeled margin, rewards, provenance and explicit gaps", () => {
  render(
    <ReportEvidence
      eventId="event-report-a"
      role="operator"
      json={fixture()}
    />,
  );
  expect(screen.getByText(/-3.25 USD/)).toBeVisible();
  expect(screen.getByText(/7.25 USD/)).toBeVisible();
  expect(screen.getByText(/SIMULATED/)).toBeVisible();
  expect(screen.getByText("delivered_energy_unavailable")).toBeVisible();
  expect(screen.getByText("Delivered energy").parentElement).toHaveTextContent(
    "Unavailable",
  );
  expect(
    screen.getByText("Reserve violations prevented").parentElement,
  ).toHaveTextContent("Unavailable");
});

test("partner receives only the aggregate report shape", () => {
  render(
    <ReportEvidence
      eventId="event-report-a"
      role="partner"
      json={fixture(".partner")}
    />,
  );
  expect(screen.getByText("Requested power").parentElement).toHaveTextContent(
    "1.000 MW",
  );
  expect(screen.queryByText(/Member rewards/)).not.toBeInTheDocument();
  expect(screen.queryByText(/policy-fixture/)).not.toBeInTheDocument();
});

test.each(["{}", "not json"])("rejects invalid report %s", (json) => {
  render(
    <ReportEvidence eventId="event-report-a" role="operator" json={json} />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Report evidence is invalid",
  );
});

test("a response for another event cannot be displayed", () => {
  render(
    <ReportEvidence eventId="other-event" role="operator" json={fixture()} />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Report evidence is invalid",
  );
  expect(screen.queryByText(/-3.25 USD/)).not.toBeInTheDocument();
});

test("partner rejects a private report response", () => {
  render(
    <ReportEvidence eventId="event-report-a" role="partner" json={fixture()} />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Report evidence is invalid",
  );
  expect(screen.queryByText(/REWARD_LEDGER/)).not.toBeInTheDocument();
});

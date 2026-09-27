import { readFileSync } from "node:fs";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { z } from "zod";
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
  const { container } = render(
    <ReportEvidence
      eventId="event-report-a"
      role="operator"
      json={fixture()}
    />,
  );
  container.querySelector("details.evidence-drawer")?.setAttribute("open", "");
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

test("zero coverage withholds delivery while measured zero remains valid", () => {
  const report = z.record(z.string(), z.unknown()).parse(JSON.parse(fixture()));
  const delivered = {
    DeliveredMW: 0,
    DeliveredMWh: 0,
    Completeness: 1,
    TrackingErrorMW: -1,
    ResponseLatency: 0,
  };
  const json = (completeness: number) =>
    JSON.stringify({
      ...report,
      DataGaps: [],
      Delivered: { ...delivered, Completeness: completeness },
    });
  const { rerender } = render(
    <ReportEvidence eventId="event-report-a" role="operator" json={json(1)} />,
  );
  expect(screen.getByText("Delivered power").parentElement).toHaveTextContent(
    "0.000 MW",
  );
  expect(screen.getByText("Delivered energy").parentElement).toHaveTextContent(
    "0.000 MWh",
  );
  expect(screen.getByText("Tracking error").parentElement).toHaveTextContent(
    "-1.000 MW",
  );
  rerender(
    <ReportEvidence eventId="event-report-a" role="operator" json={json(0)} />,
  );
  for (const label of [
    "Delivered power",
    "Delivered energy",
    "Tracking error",
  ]) {
    expect(screen.getByText(label).parentElement).toHaveTextContent(
      "Unavailable",
    );
  }
  expect(
    screen.getByText("Measurement completeness").parentElement,
  ).toHaveTextContent("0.000 %");
});

test("response latency requires an observed response count", () => {
  const report = z.record(z.string(), z.unknown()).parse(JSON.parse(fixture()));
  const json = (responded: number | undefined) =>
    JSON.stringify({
      ...report,
      Delivered: {
        DeliveredMW: 0,
        DeliveredMWh: 0,
        Completeness: 1,
        TrackingErrorMW: -1,
        ResponseLatency: 0,
        Responded: responded,
        Commanded: 1,
      },
    });
  const { rerender } = render(
    <ReportEvidence eventId="event-report-a" role="operator" json={json(1)} />,
  );
  expect(screen.getByText("Response latency").parentElement).toHaveTextContent(
    "0.000 s",
  );
  for (const responded of [0, undefined]) {
    rerender(
      <ReportEvidence
        eventId="event-report-a"
        role="operator"
        json={json(responded)}
      />,
    );
    expect(
      screen.getByText("Response latency").parentElement,
    ).toHaveTextContent("Unavailable");
  }
});

test("partner delivery requires coverage and preserves measured zero", () => {
  const report = z
    .record(z.string(), z.unknown())
    .parse(JSON.parse(fixture(".partner")));
  const json = (coverage: number | undefined) =>
    JSON.stringify({
      ...report,
      delivery_coverage: coverage,
      delivered_mw: 0,
      delivered_mwh: 0,
    });
  const { rerender } = render(
    <ReportEvidence eventId="event-report-a" role="partner" json={json(1)} />,
  );
  expect(screen.getByText("Delivered power").parentElement).toHaveTextContent(
    "0.000 MW",
  );
  for (const coverage of [0, undefined]) {
    rerender(
      <ReportEvidence
        eventId="event-report-a"
        role="partner"
        json={json(coverage)}
      />,
    );
    expect(screen.getByText("Delivered power").parentElement).toHaveTextContent(
      "Unavailable",
    );
    expect(
      screen.getByText("Delivered energy").parentElement,
    ).toHaveTextContent("Unavailable");
  }
});

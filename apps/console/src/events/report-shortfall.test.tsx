import { readFileSync } from "node:fs";
import { render, screen, within } from "@testing-library/react";
import { expect, test } from "vitest";
import { z } from "zod";
import { ReportEvidence } from "./report";

const envelope = z
  .object({ reportJson: z.string() })
  .parse(
    JSON.parse(
      readFileSync(
        "../../testdata/fixtures/api/ReportService/GetEventReport.json",
        "utf8",
      ),
    ),
  );
const recorded = z
  .record(z.string(), z.unknown())
  .parse(JSON.parse(envelope.reportJson));
const interval = { begin: "2026-09-27T12:00:00Z", end: "2026-09-27T12:15:00Z" };
const planned = {
  ...interval,
  requested_kw: 1000,
  feasible_kw: 800,
  shortfall_kw: 200,
  reasons: ["RESERVE"],
};
const delivered = {
  ...interval,
  requested_kwh: 250,
  measured_delivered_kwh: 125,
  shortfall_kwh: 125,
  coverage: 0.5,
  value_kind: "MEASURED",
};
const json = (values: Record<string, unknown>) =>
  JSON.stringify({ ...recorded, ...values });

test("report separates planned power shortfall from measured energy shortfall", () => {
  render(
    <ReportEvidence
      eventId="event-report-a"
      role="operator"
      json={json({
        planned_shortfall: [planned],
        delivery_shortfall: [delivered],
      })}
    />,
  );
  const plan = screen.getByRole("table", { name: "Planned shortfall" });
  expect(plan).toHaveTextContent("200.000 kW");
  expect(plan).toHaveTextContent("RESERVE");
  const delivery = screen.getByRole("table", { name: "Delivery shortfall" });
  expect(delivery).toHaveTextContent("125.000 kWh");
  expect(delivery).toHaveTextContent("50.0%");
  expect(delivery).toHaveTextContent("MEASURED");
  expect(
    screen.getByText(/Partial coverage does not establish full delivery/),
  ).toBeVisible();
});

test("unknown delivery is absent evidence and measured overdelivery keeps its sign", () => {
  const { rerender } = render(
    <ReportEvidence
      eventId="event-report-a"
      role="operator"
      json={json({
        delivery_shortfall: [
          {
            ...delivered,
            measured_delivered_kwh: 255,
            shortfall_kwh: -5,
            coverage: 1,
          },
        ],
      })}
    />,
  );
  expect(
    screen.getByRole("table", { name: "Delivery shortfall" }),
  ).toHaveTextContent("-5.000 kWh");
  rerender(
    <ReportEvidence
      eventId="event-report-a"
      role="operator"
      json={json({
        delivery_shortfall: [
          {
            ...interval,
            requested_kwh: 250,
            coverage: 0,
            value_kind: "UNKNOWN",
          },
        ],
      })}
    />,
  );
  const table = screen.getByRole("table", { name: "Delivery shortfall" });
  expect(within(table).getAllByText("Unavailable")).toHaveLength(2);
  expect(table).toHaveTextContent("0.0%");
  expect(table).toHaveTextContent("UNKNOWN");
});

test.each([
  { ...delivered, coverage: 0 },
  { ...delivered, coverage: 1.1 },
  { ...delivered, end: interval.begin },
  { ...delivered, shortfall_kwh: undefined },
  { ...delivered, value_kind: "UNKNOWN" },
])("invalid delivery shortfall stays outside the evidence table", (value) => {
  const { rerender } = render(
    <ReportEvidence
      eventId="event-report-a"
      role="operator"
      json={json({ delivery_shortfall: [delivered] })}
    />,
  );
  expect(
    screen.getByRole("table", { name: "Delivery shortfall" }),
  ).toBeVisible();
  rerender(
    <ReportEvidence
      eventId="event-report-a"
      role="operator"
      json={json({ delivery_shortfall: [value] })}
    />,
  );
  expect(
    screen.queryByRole("table", { name: "Delivery shortfall" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Delivery shortfall evidence is invalid",
  );
});

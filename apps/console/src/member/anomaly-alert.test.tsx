import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { HomeActivityAlertSchema } from "../api/gen/gridos/v1/pricing_pb";
import { AnomalyEvidence } from "./anomaly-alert";

const alert = create(HomeActivityAlertSchema, {
  alertId: "alert-1",
  memberId: "member-1",
  description: "energy anomaly signal",
  observedAt: timestampFromDate(new Date("2026-09-27T12:00:00Z")),
  consentVersion: "consent-1",
});

test("anomaly card retains fixed wording and consent evidence", () => {
  render(<AnomalyEvidence memberId="member-1" alerts={[alert]} />);
  expect(screen.getByText("energy anomaly signal")).toBeVisible();
  expect(screen.getByText(/consent-1/)).toBeVisible();
  expect(screen.getByText(/not a security monitoring service/)).toBeVisible();
});

test.each([{ memberId: "member-2" }, { description: "Intruder detected" }])(
  "rejects unrelated or misleading alerts: %j",
  (change) => {
    render(
      <AnomalyEvidence
        memberId="member-1"
        alerts={[create(HomeActivityAlertSchema, { ...alert, ...change })]}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Home activity evidence is invalid",
    );
    expect(screen.queryByText("energy anomaly signal")).not.toBeInTheDocument();
  },
);

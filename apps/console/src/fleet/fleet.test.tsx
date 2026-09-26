import { readFileSync } from "node:fs";
import { fromJsonString } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { GetFleetSummaryResponseSchema } from "../api/gen/gridos/v1/api_pb";
import { FleetMetrics } from "./fleet";

test("fleet presents recorded quantities with evidence and distinct operating states", () => {
  const response = fromJsonString(
    GetFleetSummaryResponseSchema,
    readFileSync(
      new URL(
        "../../../../testdata/fixtures/api/FleetService/GetFleetSummary.json",
        import.meta.url,
      ),
      "utf8",
    ),
  );
  render(<FleetMetrics summary={response.summary} />);
  expect(screen.getByText("Installed power")).toBeVisible();
  expect(screen.getByText("Reserved for backup")).toBeVisible();
  expect(screen.getByText("Telemetry unavailable")).toBeVisible();
  expect(screen.getByText("Off-grid outage")).toBeVisible();
  expect(screen.getAllByText("SIMULATED").length).toBeGreaterThan(0);
});

import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import { FleetQuantityAggregateSchema } from "./gen/gridos/v1/api_pb";
import { DataProvenance } from "./gen/gridos/v1/device_pb";
import { Quantity, provenanceNames } from "./Provenance";

const metadata = {
  timestamp: { seconds: 1786575300n, nanos: 0 },
  freshness: { seconds: 2n, nanos: 0 },
  provenanceMix: [{ provenance: DataProvenance.SIMULATED, recordCount: 5000n }],
};

describe("Provenance", () => {
  test.each(Object.keys(provenanceNames).map(Number))(
    "renders provenance class %s and observation evidence",
    (provenance) => {
      render(
        <Quantity
          label="Installed power"
          unit="MW"
          aggregate={create(FleetQuantityAggregateSchema, {
            value: 42.63,
            metadata: {
              ...metadata,
              provenanceMix: [{ provenance, recordCount: 5000n }],
            },
          })}
        />,
      );
      expect(screen.getByText("42.630")).toBeVisible();
      expect(
        screen.getByText(
          provenanceNames[provenance as keyof typeof provenanceNames],
        ),
      ).toBeVisible();
      expect(screen.getByText(/2s old at observation/)).toBeVisible();
      expect(screen.getByText(/2026-08-12/)).toBeVisible();
    },
  );

  test.each([
    { ...metadata, timestamp: undefined },
    { ...metadata, freshness: undefined },
    { ...metadata, provenanceMix: [] },
    { ...metadata, freshness: { seconds: -1n, nanos: 0 } },
    { ...metadata, provenanceMix: [{ provenance: 0, recordCount: 1n }] },
  ])(
    "refuses a numeric aggregate when evidence is invalid",
    (invalidMetadata) => {
      render(
        <Quantity
          label="Installed power"
          unit="MW"
          aggregate={create(FleetQuantityAggregateSchema, {
            value: 42.63,
            metadata: invalidMetadata,
          })}
        />,
      );
      expect(screen.getByText("Installed power")).toBeVisible();
      expect(screen.getByText("Evidence unavailable")).toBeVisible();
      expect(screen.queryByText("42.630")).not.toBeInTheDocument();
    },
  );
});

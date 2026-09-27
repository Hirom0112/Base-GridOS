import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { DrilldownResponseSchema } from "../api/gen/gridos/v1/geo_pb";
import { HierarchyEvidence } from "./hierarchy";

const metadata = {
  timestamp: { seconds: 1790424000n },
  freshness: {},
  provenanceMix: [{ provenance: 5, recordCount: 12n }],
};
const data = create(DrilldownResponseSchema, {
  metadata,
  nodes: [
    {
      id: "market:ERCOT",
      parentId: "",
      label: "SIMULATED ERCOT",
      level: 1,
      siteCount: 12n,
      metadata,
    },
  ],
});

test("hierarchy navigation preserves simulation labels and source evidence", () => {
  render(<HierarchyEvidence data={data} parentId="" open={vi.fn()} />);
  expect(screen.getByRole("button", { name: "SIMULATED ERCOT" })).toBeVisible();
  expect(screen.getByText("12 sites")).toBeVisible();
  expect(screen.getByText("SIMULATED")).toBeVisible();
});

test("hierarchy rejects unrelated nodes", () => {
  render(
    <HierarchyEvidence data={data} parentId="utility:other" open={vi.fn()} />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Hierarchy evidence is invalid",
  );
  expect(screen.queryByRole("button")).not.toBeInTheDocument();
});

import { readFileSync } from "node:fs";
import { fromJsonString } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { GetMarketContextResponseSchema } from "../api/gen/gridos/v1/api_pb";
import { MarketEvidence } from "./market";

function fixture() {
  return fromJsonString(
    GetMarketContextResponseSchema,
    readFileSync(
      "../../testdata/fixtures/api/ContextService/GetMarketContext.json",
      "utf8",
    ),
  );
}

test("reference markets retain their actual geography and old source dates", () => {
  render(<MarketEvidence data={fixture()} />);
  expect(
    screen.getByRole("table", { name: "Day-ahead reference prices" }),
  ).toHaveTextContent("HB_HOUSTON");
  expect(
    screen.getByRole("table", { name: "Real-time reference prices" }),
  ).toHaveTextContent("USD/MWh");
  expect(
    screen.getByRole("table", { name: "Reference system load" }),
  ).toHaveTextContent("NORTH_C");
  expect(screen.getAllByText(/2025-01-/).length).toBeGreaterThan(0);
  expect(screen.getByText(/LZ_AEN.*missing/)).toBeVisible();
});

test("invalid price evidence cannot display reference values", () => {
  const data = fixture();
  data.dayAheadPrices[0]!.usdPerMwh = Infinity;
  render(<MarketEvidence data={data} />);
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Market evidence is invalid",
  );
  expect(screen.queryByRole("table")).not.toBeInTheDocument();
});

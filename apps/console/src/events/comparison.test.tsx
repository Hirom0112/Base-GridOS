import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { CompareEventReportsResponseSchema } from "../api/gen/gridos/v1/api_pb";
import { ComparisonEvidence, comparisonSchema } from "./comparison";

test("comparison preserves server fields and before/after values", () => {
  render(
    <ComparisonEvidence
      data={create(CompareEventReportsResponseSchema, {
        differences: [
          { field: "margin.value_usd", before: "-3.25", after: "1.5" },
        ],
      })}
    />,
  );
  expect(screen.getByRole("table")).toHaveTextContent("margin.value_usd");
  expect(screen.getByRole("table")).toHaveTextContent("-3.25");
  expect(screen.getByRole("table")).toHaveTextContent("1.5");
});

test("comparison rejects invalid identifiers and unsafe version numbers", () => {
  expect(
    comparisonSchema.safeParse({
      eventIdA: "event-a",
      eventIdB: "event-b",
      planVersionA: "1",
      planVersionB: "2",
    }).success,
  ).toBe(true);
  expect(
    comparisonSchema.safeParse({
      eventIdA: "",
      eventIdB: "event-b",
      planVersionA: "",
      planVersionB: "",
    }).success,
  ).toBe(false);
  expect(
    comparisonSchema.safeParse({
      eventIdA: "event-a",
      eventIdB: "event-b",
      planVersionA: "18446744073709551616",
      planVersionB: "",
    }).success,
  ).toBe(false);
});

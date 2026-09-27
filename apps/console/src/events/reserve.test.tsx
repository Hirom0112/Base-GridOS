import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { ReserveEvidence } from "./reserve";

const evidence = {
  DevicesExpected: 100,
  DevicesObserved: 99,
  MinimumMarginKWh: -0.25,
  DevicesTouchedFloor: 1,
  ObservationGaps: 1,
  ValueKind: "MEASURED",
  Provenance: ["TELEMETRY_OBSERVATIONS", "FROZEN_EFFECTIVE_RESERVE"],
};

test("measured reserve margins retain breaches and observation gaps", () => {
  render(<ReserveEvidence evidence={evidence} />);
  const reserve = screen.getByRole("region", {
    name: "Reserve protection evidence",
  });
  expect(reserve).toHaveTextContent("-0.250 kWh");
  expect(reserve).toHaveTextContent("99 of 100 devices observed");
  expect(reserve).toHaveTextContent("1 devices with observation gaps");
  expect(reserve).toHaveTextContent(
    "1 devices touched or crossed the reserve floor",
  );
  expect(reserve).toHaveTextContent("FROZEN_EFFECTIVE_RESERVE");
  expect(reserve).toHaveTextContent("Unobserved devices remain unknown");
});

test("missing reserve measurements never imply compliance", () => {
  render(<ReserveEvidence evidence={null} />);
  expect(
    screen.getByRole("region", { name: "Reserve protection evidence" }),
  ).toHaveTextContent("Measured reserve evidence unavailable");
  expect(screen.queryByText(/0.000 kWh/)).not.toBeInTheDocument();
});

test("inconsistent reserve observation counts fail closed", () => {
  render(<ReserveEvidence evidence={{ ...evidence, ObservationGaps: 0 }} />);
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Reserve evidence is invalid",
  );
  expect(screen.queryByText(/-0.250 kWh/)).not.toBeInTheDocument();
});

test("a device can be observed and still have gaps in the event window", () => {
  render(
    <ReserveEvidence
      evidence={{
        ...evidence,
        DevicesObserved: 100,
        ObservationGaps: 1,
        MinimumMarginKWh: 0.25,
      }}
    />,
  );
  expect(
    screen.getByRole("region", { name: "Reserve protection evidence" }),
  ).toHaveTextContent("100 of 100 devices observed");
  expect(
    screen.getByRole("region", { name: "Reserve protection evidence" }),
  ).toHaveTextContent("1 devices with observation gaps");
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

test.each([-0.25, -0.0001])(
  "negative reserve margin %s has an explicit breach warning",
  (margin) => {
    const view = render(
      <ReserveEvidence evidence={{ ...evidence, MinimumMarginKWh: 0.25 }} />,
    );
    expect(
      screen.getByRole("region", { name: "Reserve protection evidence" }),
    ).toHaveTextContent("0.250 kWh");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    view.rerender(
      <ReserveEvidence evidence={{ ...evidence, MinimumMarginKWh: margin }} />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Reserve breach observed",
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "before another dispatch",
    );
  },
);

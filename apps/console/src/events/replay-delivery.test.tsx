import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { ListEventCommandsResponseSchema } from "../api/gen/gridos/v1/events_pb";
import { ReplayDeliveryEvidence } from "./replay-delivery";

const at = timestampFromDate(new Date("2026-09-27T12:01:00Z"));
function fixture() {
  return create(ListEventCommandsResponseSchema, {
    verificationIntervals: [1, 2].map((minute) => ({
      beginTime: timestampFromDate(
        new Date(`2026-09-27T12:0${minute - 1}:00Z`),
      ),
      endTime: timestampFromDate(new Date(`2026-09-27T12:0${minute}:00Z`)),
      requestedKw: 4,
      commandedKw: 3,
      deliveredKw: minute === 1 ? -1 : 999,
      trackingErrorKw: minute === 1 ? -5 : 995,
      confidence: 0.9,
      measurementBoundary: "METER_NET_EXPORT",
      baselineMethod: "DIRECT",
      valueKind: "MEASURED",
    })),
  });
}

test("the replay chart includes only completed measured intervals at the shared time", () => {
  render(<ReplayDeliveryEvidence data={fixture()} asOf={at} />);
  expect(screen.getByRole("table")).toHaveTextContent("-1 kW");
  expect(screen.getByRole("table")).not.toHaveTextContent("999");
  expect(
    screen.getByRole("img", {
      name: "Replay requested commanded and measured power",
    }),
  ).toBeVisible();
  expect(screen.getByText(/Publication times are not supplied/)).toBeVisible();
});

test("missing intervals do not become zero delivery", () => {
  render(
    <ReplayDeliveryEvidence
      data={fixture()}
      asOf={timestampFromDate(new Date("2026-09-27T11:59:00Z"))}
    />,
  );
  expect(
    screen.getByText(
      "No measured intervals end by this replay time. Delivery is unavailable.",
    ),
  ).toBeVisible();
  expect(screen.queryByRole("img")).not.toBeInTheDocument();
});

test("invalid measurement evidence blocks the replay chart", () => {
  const data = fixture();
  data.verificationIntervals[0]!.valueKind = "MODELED";
  render(<ReplayDeliveryEvidence data={data} asOf={at} />);
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Invalid replay measurement evidence",
  );
});

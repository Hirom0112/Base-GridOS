import { create } from "@bufbuild/protobuf";
import { TimestampSchema } from "@bufbuild/protobuf/wkt";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import {
  WatchEventResponseSchema,
  EventPowerAggregateSchema,
} from "../api/gen/gridos/v1/api_pb";
import { UncertaintyIntervalSchema } from "../api/gen/gridos/v1/verification_pb";
import { reduceEventSamples, EventResponse } from "./events-live";

const update = create(WatchEventResponseSchema, {
  event: { eventId: "event-1", state: 6 },
  observedAt: { seconds: 1790424000n, nanos: 0 },
  fleet: {
    sentMw: 1,
    acknowledgedMw: 0.8,
    deliveredMw: 0.6,
    deliveredState: 1,
    metadata: {
      timestamp: { seconds: 1790424000n },
      freshness: { seconds: 0n },
      provenanceMix: [{ provenance: 5, recordCount: 10n }],
    },
  },
});

test("live response separates sent, acknowledgement and measured power", () => {
  const samples = reduceEventSamples([], update, "event-1");
  render(<EventResponse samples={samples} />);
  expect(screen.getByText("1.000 MW")).toBeVisible();
  expect(screen.getByText("0.800 MW")).toBeVisible();
  expect(screen.getByText("0.600 MW")).toBeVisible();
  expect(screen.getByText("SIMULATED")).toBeVisible();
});

test("unknown delivery is a gap and never becomes zero", () => {
  const unknown = create(WatchEventResponseSchema, {
    ...update,
    fleet: create(EventPowerAggregateSchema, {
      ...update.fleet!,
      deliveredMw: 0,
      deliveredState: 4,
    }),
  });
  const samples = reduceEventSamples([], unknown, "event-1");
  expect(samples[0]?.delivered).toBeNull();
  render(<EventResponse samples={samples} />);
  expect(screen.getByText("Delivery unknown")).toBeVisible();
  expect(screen.queryByText("0.000 MW")).not.toBeInTheDocument();
});

test("stream samples reject foreign events and invalid evidence", () => {
  expect(() => reduceEventSamples([], update, "event-1")).not.toThrow();
  expect(() => reduceEventSamples([], update, "other")).toThrow();
  expect(() =>
    reduceEventSamples(
      [],
      create(WatchEventResponseSchema, {
        ...update,
        fleet: create(EventPowerAggregateSchema, {
          ...update.fleet!,
          metadata: undefined,
        }),
      }),
      "event-1",
    ),
  ).toThrow();
});

test("sample history is bounded and duplicate timestamps replace the last sample", () => {
  let samples = reduceEventSamples([], update, "event-1");
  samples = reduceEventSamples(samples, update, "event-1");
  expect(samples).toHaveLength(1);
  for (let index = 1; index < 125; index++)
    samples = reduceEventSamples(
      samples,
      create(WatchEventResponseSchema, {
        ...update,
        observedAt: create(TimestampSchema, {
          seconds: 1790424000n + BigInt(index),
          nanos: 0,
        }),
      }),
      "event-1",
    );
  expect(samples).toHaveLength(120);
  expect(() => reduceEventSamples(samples, update, "event-1")).toThrow();
});

test("uncertainty evidence rejects a reversed time interval", () => {
  const withInterval = (begin: bigint, end: bigint) =>
    create(WatchEventResponseSchema, {
      ...update,
      fleet: create(EventPowerAggregateSchema, {
        ...update.fleet!,
        uncertaintyIntervals: [
          create(UncertaintyIntervalSchema, {
            intervalBeginTime: { seconds: begin, nanos: 0 },
            intervalEndTime: { seconds: end, nanos: 0 },
            signedFeasiblePowerLowerKw: 0,
            signedFeasiblePowerUpperKw: 1,
          }),
        ],
      }),
    });
  expect(() =>
    reduceEventSamples([], withInterval(1n, 2n), "event-1"),
  ).not.toThrow();
  expect(() =>
    reduceEventSamples([], withInterval(2n, 1n), "event-1"),
  ).toThrow();
});

test("stream keeps independently measured H3 evidence", () => {
  const spatial = create(WatchEventResponseSchema, {
    ...update,
    h3: [
      {
        h3Cell: "87489d884ffffff",
        power: update.fleet,
        metadata: update.fleet!.metadata,
      },
    ],
  });
  const samples = reduceEventSamples([], spatial, "event-1");
  expect(samples[0]?.h3[0]?.h3Cell).toBe("87489d884ffffff");
  expect(samples[0]?.h3[0]?.power?.deliveredMw).toBe(0.6);
  const invalid = create(WatchEventResponseSchema, {
    ...spatial,
    h3: [{ ...spatial.h3[0]!, h3Cell: "invalid" }],
  });
  expect(() => reduceEventSamples([], invalid, "event-1")).toThrow();
});

test("zero per-cell delivery cannot carry a nonzero measurement", () => {
  const spatial = create(WatchEventResponseSchema, {
    ...update,
    h3: [
      {
        h3Cell: "87489d884ffffff",
        power: { ...update.fleet!, deliveredState: 5, deliveredMw: 0.2 },
        metadata: update.fleet!.metadata,
      },
    ],
  });
  expect(() => reduceEventSamples([], spatial, "event-1")).toThrow();
});

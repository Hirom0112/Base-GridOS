import { expect, test } from "vitest";
import { travelWindow } from "./travel-time";

test("converts the selected household timezone without using the browser timezone", () => {
  expect(
    travelWindow("2026-09-28T10:00", "2026-09-28T12:00", "America/Chicago"),
  ).toEqual({
    startTime: { seconds: 1790607600n, nanos: 0 },
    endTime: { seconds: 1790614800n, nanos: 0 },
    timezone: "America/Chicago",
  });
});

test.each([
  ["2026-03-08T02:30", "2026-03-08T04:00", "America/Chicago"],
  ["2026-11-01T01:30", "2026-11-01T04:00", "America/Chicago"],
  ["2026-09-28T12:00", "2026-09-28T10:00", "America/Chicago"],
  ["2026-09-28T10:00", "2026-09-28T10:00", "America/Chicago"],
  ["2026-09-28T10:00", "2026-09-28T12:00", "+05:00"],
  ["2026-09-28T10:00", "2026-09-28T12:00", "Invalid/Zone"],
])(
  "rejects invalid or ambiguous travel times %s %s %s",
  (start, end, timezone) => {
    expect(() => travelWindow(start, end, timezone)).toThrow();
  },
);

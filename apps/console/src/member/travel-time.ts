import { Temporal } from "@js-temporal/polyfill";
import { z } from "zod";

const localTime = z.iso.datetime({ local: true, precision: -1 });
const timeZone = z
  .string()
  .min(1)
  .refine((value) => !/^[+-]/.test(value));

export function travelWindow(start: string, end: string, timezone: string) {
  const zone = timeZone.parse(timezone);
  const options = { disambiguation: "reject", offset: "reject" } as const;
  const startInstant = Temporal.ZonedDateTime.from(
    `${localTime.parse(start)}[${zone}]`,
    options,
  ).toInstant();
  const endInstant = Temporal.ZonedDateTime.from(
    `${localTime.parse(end)}[${zone}]`,
    options,
  ).toInstant();
  if (Temporal.Instant.compare(startInstant, endInstant) >= 0)
    throw new Error("Travel must end after it starts.");
  return {
    startTime: {
      seconds: startInstant.epochNanoseconds / 1_000_000_000n,
      nanos: 0,
    },
    endTime: {
      seconds: endInstant.epochNanoseconds / 1_000_000_000n,
      nanos: 0,
    },
    timezone: zone,
  };
}

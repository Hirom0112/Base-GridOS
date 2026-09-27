import { z } from "zod";
import type {
  MemberOffer,
  MemberOfferTerms,
} from "../api/gen/gridos/v1/member_pb";

const text = z.string().min(1);
const percent = z.number().min(0).max(100);
const cents = z.bigint().min(0n);
const timestamp = z.object({
  seconds: z.bigint(),
  nanos: z.number().int().min(0).max(999999999),
});
const binding = z.object({
  kind: z.union([z.literal(1), z.literal(2)]),
  market: text,
  catalogVersion: text,
  memberPlanId: text,
  contractVersion: text,
  consentVersion: text,
  consentText: text,
  priceText: text,
  temporaryReservePercent: percent.optional(),
  creditType: z.union([z.literal(0), z.literal(1), z.literal(2), z.literal(3)]),
  fixedCreditCents: cents,
  energyMonthlyChargeCents: cents,
  batteryMonthlyChargeCents: cents,
  flexibilityRewardCents: cents,
  effectiveAt: timestamp,
  expiresAt: timestamp,
});

export const offerTerms = binding
  .extend({
    displayName: text,
    reserveFloorPercent: percent,
    policyVersion: text,
  })
  .refine((value) => value.expiresAt.seconds > value.effectiveAt.seconds)
  .refine(
    (value) =>
      value.kind !== 2 ||
      (value.temporaryReservePercent !== undefined && value.creditType !== 0),
  );

export function matchingOffer(
  receipt: MemberOffer | undefined,
  terms: MemberOfferTerms,
  memberId: string,
) {
  const parsed = binding
    .extend({ offerId: text, memberId: z.literal(memberId) })
    .parse(receipt);
  const expected = binding.parse(terms);
  for (const key of Object.keys(expected) as (keyof typeof expected)[]) {
    const actual = parsed[key];
    const value = expected[key];
    if (typeof value === "object" && typeof actual === "object") {
      if (value.seconds === actual.seconds && value.nanos === actual.nanos)
        continue;
    } else if (value === actual) continue;
    throw new Error(
      "The recorded offer differs from the terms shown. Selection is blocked.",
    );
  }
  return parsed;
}

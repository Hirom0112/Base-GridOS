import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { expect, test } from "vitest";
import {
  MemberOfferSchema,
  MemberOfferTermsSchema,
} from "../api/gen/gridos/v1/member_pb";
import { offerTerms, matchingOffer } from "./offer-terms";

function terms() {
  return create(MemberOfferTermsSchema, {
    kind: 1,
    market: "ERCOT",
    catalogVersion: "catalog-1",
    memberPlanId: "balanced",
    displayName: "Balanced",
    reserveFloorPercent: 30,
    policyVersion: "policy-2",
    contractVersion: "contract-1",
    consentVersion: "consent-1",
    consentText: "Exact consent",
    priceText: "Exact price",
    effectiveAt: timestampFromDate(new Date("2026-01-01T00:00:00Z")),
    expiresAt: timestampFromDate(new Date("2027-01-01T00:00:00Z")),
  });
}

test("accepts complete stored terms and their matching presented offer", () => {
  const source = terms();
  expect(offerTerms.parse(source).policyVersion).toBe("policy-2");
  const receipt = create(MemberOfferSchema, {
    ...source,
    $typeName: undefined,
    memberId: "member-1",
    offerId: "offer-1",
  });
  expect(matchingOffer(receipt, source, "member-1").offerId).toBe("offer-1");
  expect(() =>
    matchingOffer({ ...receipt, memberId: "other" }, source, "member-1"),
  ).toThrow();
  expect(() =>
    matchingOffer(
      { ...receipt, priceText: "Different price" },
      source,
      "member-1",
    ),
  ).toThrow();
  expect(() =>
    matchingOffer(
      { ...receipt, energyMonthlyChargeCents: 100n },
      source,
      "member-1",
    ),
  ).toThrow();
});

test("rejects missing linkage, unknown kinds, invalid reserves and unbounded terms", () => {
  for (const change of [
    { policyVersion: "" },
    { kind: 0 },
    { reserveFloorPercent: 101 },
    { expiresAt: undefined },
  ])
    expect(offerTerms.safeParse({ ...terms(), ...change }).success).toBe(false);
  expect(
    offerTerms.safeParse({
      ...terms(),
      kind: 2,
      temporaryReservePercent: 20,
      creditType: 1,
    }).success,
  ).toBe(true);
  expect(offerTerms.safeParse({ ...terms(), kind: 2 }).success).toBe(false);
});

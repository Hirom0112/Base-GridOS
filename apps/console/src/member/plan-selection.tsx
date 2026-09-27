import { useRef, useState } from "react";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { z } from "zod";
import { useSession } from "../api/auth";
import {
  SelectResiliencePlanRequestSchema,
  type SelectResiliencePlanRequest,
  type MemberOfferTerms,
} from "../api/gen/gridos/v1/member_pb";

export function PlanSelection({
  terms,
  memberId,
  offerId,
  onConfirmed,
  onDismiss,
}: {
  terms: MemberOfferTerms;
  memberId: string;
  offerId: string;
  onConfirmed: () => void;
  onDismiss?: () => void;
}) {
  const { client } = useSession();
  const [consent, setConsent] = useState(false);
  const [state, setState] = useState<
    "ready" | "pending" | "unknown" | "confirmed"
  >("ready");
  const [error, setError] = useState("");
  const pending = useRef(false);
  const intent = useRef<SelectResiliencePlanRequest | null>(null);
  const explanation = `Your plan reserves ${terms.reserveFloorPercent}% for backup. Weather and safety protections may keep the effective reserve higher.${terms.reserveFloorPercent === 0 ? " A 0% plan reserve does not remove battery hardware limits or guarantee backup availability." : ""}`;
  async function select() {
    if (pending.current || !consent || state === "confirmed") return;
    if (!intent.current) {
      if (
        !terms.expiresAt ||
        Number(terms.expiresAt.seconds) * 1000 <= Date.now()
      ) {
        setError("This offer expired. Refresh offers before selecting a plan.");
        return;
      }
      intent.current = create(SelectResiliencePlanRequestSchema, {
        memberId,
        offerId,
        idempotencyKey: crypto.randomUUID(),
        correlationId: crypto.randomUUID(),
        market: terms.market,
        catalogVersion: terms.catalogVersion,
        memberPlanId: terms.memberPlanId,
        policyVersion: terms.policyVersion,
        consentText: terms.consentText,
        consentVersion: terms.consentVersion,
        explanationShown: explanation,
        effectiveAt: timestampFromDate(new Date()),
      });
    }
    pending.current = true;
    setState("pending");
    setError("");
    try {
      const { plan } = await client.member.selectResiliencePlan(intent.current);
      z.object({
        selectionId: z.string().min(1),
        offerId: z.literal(offerId),
        memberPlanId: z.literal(terms.memberPlanId),
        catalogVersion: z.literal(terms.catalogVersion),
        policyVersion: z.literal(terms.policyVersion),
        reserveFloorPercent: z.literal(terms.reserveFloorPercent),
        termsKnown: z.literal(true),
        energyMonthlyChargeCents: z.literal(terms.energyMonthlyChargeCents),
        batteryMonthlyChargeCents: z.literal(terms.batteryMonthlyChargeCents),
        flexibilityRewardCents: z.literal(terms.flexibilityRewardCents),
        effectiveAt: z.object({
          seconds: z.literal(intent.current.effectiveAt!.seconds),
          nanos: z.literal(intent.current.effectiveAt!.nanos),
        }),
      }).parse(plan);
      setState("confirmed");
      onConfirmed();
    } catch (cause) {
      setState("unknown");
      setError(
        `Outcome unknown. Retry the same selection to reconcile it. ${cause instanceof Error ? cause.message : "No matching receipt received."}`,
      );
    } finally {
      pending.current = false;
    }
  }
  return (
    <div className="member-consent">
      <p>{explanation}</p>
      <p>{terms.consentText}</p>
      <label>
        <input
          type="checkbox"
          checked={consent}
          disabled={state !== "ready"}
          onChange={(event) => setConsent(event.target.checked)}
        />
        I agree to the displayed plan terms.
      </label>
      {state === "ready" && onDismiss && (
        <button type="button" onClick={onDismiss}>
          Cancel review
        </button>
      )}
      {error && <p role="alert">{error}</p>}
      {state === "confirmed" ? (
        <p role="status">
          Plan selection recorded. Household status will show the effective
          reserve.
        </p>
      ) : (
        <button onClick={select} disabled={!consent || state === "pending"}>
          {state === "unknown"
            ? "Retry same selection"
            : state === "pending"
              ? "Recording selection…"
              : "Confirm plan"}
        </button>
      )}
    </div>
  );
}

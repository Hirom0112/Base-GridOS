import { useRef, useState, type ReactNode } from "react";
import { create } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useSession } from "../api/auth";
import {
  PresentOfferRequestSchema,
  type PresentOfferRequest,
  type MemberOfferTerms,
} from "../api/gen/gridos/v1/member_pb";
import { matchingOffer, offerTerms } from "./offer-terms";
import { PlanSelection } from "./plan-selection";

export function OfferReview({
  memberId,
  terms,
  onConfirmed,
  onDismiss,
  travel,
}: {
  memberId: string;
  terms: MemberOfferTerms;
  onConfirmed: () => void;
  onDismiss?: () => void;
  travel?: (offerId: string) => ReactNode;
}) {
  const { client } = useSession();
  const [state, setState] = useState<
    | { kind: "ready" }
    | { kind: "pending" }
    | { kind: "unknown"; error: string }
    | { kind: "recorded"; offerId: string }
  >({ kind: "ready" });
  const pending = useRef(false);
  const intent = useRef<PresentOfferRequest | null>(null);
  const valid = offerTerms.safeParse(terms).success;
  async function review() {
    if (!valid || pending.current || state.kind === "recorded") return;
    pending.current = true;
    setState({ kind: "pending" });
    try {
      if (!intent.current) {
        if (
          timestampDate(terms.effectiveAt!).getTime() > Date.now() ||
          timestampDate(terms.expiresAt!).getTime() <= Date.now()
        )
          throw new Error(
            "These terms are not currently effective. Refresh offers.",
          );
        intent.current = create(PresentOfferRequestSchema, {
          ...offerTerms.parse(terms),
          memberId,
          idempotencyKey: crypto.randomUUID(),
          correlationId: crypto.randomUUID(),
        });
      }
      const response = await client.member.presentOffer(intent.current);
      const receipt = matchingOffer(response.offer, terms, memberId);
      setState({ kind: "recorded", offerId: receipt.offerId });
    } catch (cause) {
      setState({
        kind: "unknown",
        error:
          cause instanceof Error
            ? cause.message
            : "No matching receipt received.",
      });
    } finally {
      pending.current = false;
    }
  }
  return (
    <section
      className="member-offer"
      aria-label={`Review ${terms.displayName}`}
    >
      <h3>
        {terms.displayName}
        {terms.kind === 2 ? " · Travel Flex" : ""}
      </h3>
      <p>{terms.priceText}</p>
      <dl className="report-quantities">
        <div>
          <dt>Plan backup reserve</dt>
          <dd>{terms.reserveFloorPercent}%</dd>
        </div>
        <div>
          <dt>Energy monthly charge</dt>
          <dd>{money(terms.energyMonthlyChargeCents)}</dd>
        </div>
        <div>
          <dt>Battery monthly charge</dt>
          <dd>{money(terms.batteryMonthlyChargeCents)}</dd>
        </div>
        <div>
          <dt>Flexibility reward</dt>
          <dd>{money(terms.flexibilityRewardCents)}</dd>
        </div>
      </dl>
      <p>
        Market {terms.market} · Catalog {terms.catalogVersion} · Contract{" "}
        {terms.contractVersion} · Consent {terms.consentVersion} · Policy{" "}
        {terms.policyVersion}
      </p>
      {terms.expiresAt && (
        <p>Terms expire {timestampDate(terms.expiresAt).toISOString()}.</p>
      )}
      {!valid && (
        <p role="alert">
          Complete, bounded offer terms are unavailable. Selection is blocked.
        </p>
      )}
      {state.kind === "ready" && onDismiss && (
        <button type="button" onClick={onDismiss}>
          Cancel review
        </button>
      )}
      {state.kind === "unknown" && (
        <p role="alert">Offer review is not confirmed. {state.error}</p>
      )}
      {state.kind !== "recorded" && (
        <button disabled={!valid || state.kind === "pending"} onClick={review}>
          {state.kind === "unknown"
            ? "Retry same offer"
            : state.kind === "pending"
              ? "Recording offer…"
              : "Review offer"}
        </button>
      )}
      {state.kind === "recorded" &&
        (terms.kind === 1 ? (
          <PlanSelection
            memberId={memberId}
            terms={terms}
            offerId={state.offerId}
            onConfirmed={onConfirmed}
            onDismiss={onDismiss}
          />
        ) : (
          travel?.(state.offerId)
        ))}
    </section>
  );
}

export function money(cents: bigint) {
  return `${cents / 100n}.${(cents % 100n).toString().padStart(2, "0")} USD`;
}

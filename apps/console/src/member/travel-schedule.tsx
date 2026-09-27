import { useRef, useState, type FormEvent } from "react";
import { create } from "@bufbuild/protobuf";
import { z } from "zod";
import { useSession } from "../api/auth";
import {
  ScheduleTravelFlexRequestSchema,
  type ScheduleTravelFlexRequest,
  type MemberOfferTerms,
} from "../api/gen/gridos/v1/member_pb";
import { travelWindow } from "./travel-time";
import { money } from "./offer-review";

const creditNames = { 1: "daily", 2: "per event", 3: "annual" } as const;
const fields = z.object({
  start: z.string(),
  end: z.string(),
  timezone: z.string(),
  earlyReturn: z.enum(["1", "2"]),
});

export function TravelSchedule({
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
  const intent = useRef<ScheduleTravelFlexRequest | null>(null);
  const pending = useRef(false);
  async function schedule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending.current || !consent || state === "confirmed") return;
    setError("");
    if (!intent.current) {
      try {
        const input = fields.parse(
          Object.fromEntries(new FormData(event.currentTarget)),
        );
        const window = travelWindow(input.start, input.end, input.timezone);
        if (Number(window.startTime.seconds) * 1000 <= Date.now())
          throw new Error("Travel must start in the future.");
        if (
          !terms.expiresAt ||
          window.endTime.seconds > terms.expiresAt.seconds
        )
          throw new Error("Travel must end before the offered terms expire.");
        if (
          terms.temporaryReservePercent === undefined ||
          terms.creditType === 0
        )
          throw new Error("Complete Travel Flex terms are unavailable.");
        intent.current = create(ScheduleTravelFlexRequestSchema, {
          ...window,
          memberId,
          offerId,
          idempotencyKey: crypto.randomUUID(),
          correlationId: crypto.randomUUID(),
          temporaryReservePercent: terms.temporaryReservePercent,
          earlyReturnAction: Number(input.earlyReturn),
          creditType: terms.creditType,
          fixedCreditCents: terms.fixedCreditCents,
          consentText: terms.consentText,
          consentVersion: terms.consentVersion,
          policyVersion: terms.policyVersion,
        });
      } catch (cause) {
        setError(
          `Check the travel window and household timezone. Daylight-saving gaps and repeated times cannot be scheduled. ${cause instanceof Error ? cause.message : "Invalid travel window."}`,
        );
        return;
      }
    }
    pending.current = true;
    setState("pending");
    try {
      const receipt = await client.member.scheduleTravelFlex(intent.current);
      z.object({
        windowId: z.string().min(1),
        temporaryReservePercent: z.literal(
          intent.current.temporaryReservePercent,
        ),
        fixedCreditCents: z.literal(intent.current.fixedCreditCents),
        startTime: z.object({
          seconds: z.literal(intent.current.startTime!.seconds),
          nanos: z.literal(intent.current.startTime!.nanos),
        }),
        endTime: z.object({
          seconds: z.literal(intent.current.endTime!.seconds),
          nanos: z.literal(intent.current.endTime!.nanos),
        }),
      }).parse(receipt);
      setState("confirmed");
      onConfirmed();
    } catch (cause) {
      setState("unknown");
      setError(
        `Outcome unknown. Retry the same travel window to reconcile it. ${cause instanceof Error ? cause.message : "No matching receipt received."}`,
      );
    } finally {
      pending.current = false;
    }
  }
  return (
    <form className="member-consent" onSubmit={schedule}>
      <p>
        Temporary backup reserve: {terms.temporaryReservePercent}%. Weather and
        safety protections may keep the effective reserve higher.
      </p>
      <p>
        Fixed credit: {money(terms.fixedCreditCents)}{" "}
        {creditNames[terms.creditType as keyof typeof creditNames]}. The server
        checks eligibility and overlapping windows.
      </p>
      <TravelFields locked={state !== "ready"} />
      <p>{terms.consentText}</p>
      <label>
        <input
          type="checkbox"
          checked={consent}
          disabled={state !== "ready"}
          onChange={(event) => setConsent(event.target.checked)}
        />
        I agree to the displayed Travel Flex terms and reserve change.
      </label>
      {state === "ready" && onDismiss && (
        <button type="button" onClick={onDismiss}>
          Cancel review
        </button>
      )}
      {error && <p role="alert">{error}</p>}
      {state === "confirmed" ? (
        <p role="status">
          Travel Flex scheduled. Your stored window appears below.
        </p>
      ) : (
        <button type="submit" disabled={!consent || state === "pending"}>
          {state === "unknown"
            ? "Retry same travel window"
            : state === "pending"
              ? "Scheduling…"
              : "Confirm Travel Flex"}
        </button>
      )}
    </form>
  );
}

function TravelFields({ locked }: { locked: boolean }) {
  return (
    <fieldset disabled={locked} className="member-travel-fields">
      <legend>Travel window</legend>
      <label>
        Travel starts
        <input name="start" type="datetime-local" required />
      </label>
      <label>
        Travel ends
        <input name="end" type="datetime-local" required />
      </label>
      <label>
        Household timezone
        <input
          name="timezone"
          defaultValue={Intl.DateTimeFormat().resolvedOptions().timeZone}
          required
        />
      </label>
      <label>
        On early return
        <select name="earlyReturn" defaultValue="1">
          <option value="1">Restore my plan reserve</option>
          <option value="2">Restore maximum reserve</option>
        </select>
      </label>
    </fieldset>
  );
}

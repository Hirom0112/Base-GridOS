import { useRef, useState } from "react";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { z } from "zod";
import { useSession } from "../api/auth";
import {
  EndTravelFlexEarlyRequestSchema,
  type EndTravelFlexEarlyRequest,
  type ScheduledTravelFlexWindow,
} from "../api/gen/gridos/v1/member_pb";

export function TravelReturn({
  memberId,
  window,
  onConfirmed,
}: {
  memberId: string;
  window: ScheduledTravelFlexWindow;
  onConfirmed: () => void;
}) {
  const { client } = useSession();
  const [consent, setConsent] = useState(false);
  const [state, setState] = useState<
    "ready" | "pending" | "unknown" | "confirmed"
  >("ready");
  const [error, setError] = useState("");
  const intent = useRef<EndTravelFlexEarlyRequest | null>(null);
  const pending = useRef(false);
  const action = { 1: "Restore my plan reserve", 2: "Restore maximum reserve" }[
    window.earlyReturnAction
  ];
  async function end() {
    if (
      pending.current ||
      !consent ||
      !action ||
      !window.consentVersion ||
      state === "confirmed"
    )
      return;
    intent.current ??= create(EndTravelFlexEarlyRequestSchema, {
      memberId,
      windowId: window.windowId,
      consentVersion: window.consentVersion,
      returnedAt: timestampFromDate(new Date()),
      idempotencyKey: crypto.randomUUID(),
      correlationId: crypto.randomUUID(),
    });
    pending.current = true;
    setState("pending");
    setError("");
    try {
      const receipt = await client.member.endTravelFlexEarly(intent.current);
      z.object({
        windowId: z.literal(window.windowId),
        returnedAt: z.object({
          seconds: z.literal(intent.current.returnedAt!.seconds),
          nanos: z.literal(intent.current.returnedAt!.nanos),
        }),
      }).parse(receipt);
      setState("confirmed");
      onConfirmed();
    } catch (cause) {
      setState("unknown");
      setError(
        `Outcome unknown. Retry the same early return to reconcile it. ${cause instanceof Error ? cause.message : "No matching receipt received."}`,
      );
    } finally {
      pending.current = false;
    }
  }
  if (window.cancelledAt) return <p>Early return recorded for this window.</p>;
  if (!action || !window.consentVersion)
    return <p>Stored early-return terms are unavailable.</p>;
  return (
    <div className="member-consent">
      <p>
        On early return: {action}. Stored consent: {window.consentVersion}.
      </p>
      <label>
        <input
          type="checkbox"
          checked={consent}
          disabled={state !== "ready"}
          onChange={(event) => setConsent(event.target.checked)}
        />
        End this Travel Flex window now.
      </label>
      {error && <p role="alert">{error}</p>}
      {state === "confirmed" ? (
        <p role="status">
          Early return recorded. Household status will show the effective
          reserve.
        </p>
      ) : (
        <button onClick={end} disabled={!consent || state === "pending"}>
          {state === "unknown"
            ? "Retry same early return"
            : state === "pending"
              ? "Recording early return…"
              : "Confirm early return"}
        </button>
      )}
    </div>
  );
}

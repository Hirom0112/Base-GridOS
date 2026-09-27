import { useEffect, useRef, useState, type FormEvent } from "react";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { z } from "zod";
import { useSession } from "../api/auth";
import { StepUpFailure } from "../api/step-up";

const reasonSchema = z.string().trim().min(1).max(1000);
type Outcome =
  | { state: "idle" }
  | { state: "pending" }
  | { state: "unknown"; error: string }
  | { state: "blocked"; error: string }
  | { state: "requested"; receipt: string };

export function EmergencyStopControl({ eventId }: { eventId: string }) {
  const { client, identity } = useSession();
  const [open, setOpen] = useState(false);
  const [outcome, setOutcome] = useState<Outcome>({ state: "idle" });
  const intent = useRef<
    Parameters<typeof client.events.emergencyStop>[0] | null
  >(null);
  const pending = useRef(false);
  async function requestStop(reason: string) {
    if (pending.current) return;
    const parsed = reasonSchema.safeParse(reason);
    if (!parsed.success)
      throw new Error("Enter a reason of 1–1,000 characters.");
    intent.current ??= {
      eventId,
      reason: parsed.data,
      requestedBy:
        identity.mode === "local" ? `local-${identity.role}` : identity.userId,
      requestedAt: timestampFromDate(new Date()),
      idempotencyKey: crypto.randomUUID(),
      correlationId: crypto.randomUUID(),
    };
    pending.current = true;
    setOutcome({ state: "pending" });
    try {
      const response = await client.events.emergencyStop(intent.current);
      const stop = response.emergencyStop;
      if (
        !response.stopRequested ||
        !stop?.emergencyStopId ||
        stop.eventId !== eventId ||
        stop.idempotencyKey !== intent.current.idempotencyKey
      )
        throw new Error("The server did not return a matching stop receipt.");
      setOutcome({ state: "requested", receipt: stop.emergencyStopId });
      setOpen(false);
    } catch (error) {
      setOutcome({
        state:
          error instanceof StepUpFailure && outcome.state !== "unknown"
            ? "blocked"
            : "unknown",
        error: error instanceof Error ? error.message : "Stop request failed",
      });
    } finally {
      pending.current = false;
    }
  }
  if (!["operator", "approver"].includes(identity.role))
    return (
      <p className="boundary-note">
        Operator or approver role required to request an emergency stop.
      </p>
    );
  return (
    <section className="stop-control" aria-label="Emergency stop">
      {outcome.state === "requested" ? (
        <div role="status">
          <strong>STOP REQUESTED</strong>
          <p>
            The fleet is not confirmed stopped. Await acknowledgement or
            measured response.
          </p>
          <small className="mono">Receipt {outcome.receipt}</small>
        </div>
      ) : (
        <>
          <div>
            <strong>Emergency stop</strong>
            <p>Request safe zero setpoints. Confirmation is required.</p>
          </div>
          <button
            className="secondary-button stop-button"
            onClick={() => setOpen(true)}
          >
            Request emergency stop
          </button>
        </>
      )}
      {!open && outcome.state === "unknown" && (
        <p role="alert">
          Stop outcome unknown. Reopen the confirmation to retry the same
          request.
        </p>
      )}
      {!open && outcome.state === "blocked" && (
        <p role="alert">Authorization failed. No stop command was sent.</p>
      )}
      {open && (
        <StopDialog
          eventId={eventId}
          outcome={outcome}
          reason={intent.current?.reason ?? ""}
          onClose={() => setOpen(false)}
          onConfirm={requestStop}
        />
      )}
    </section>
  );
}

function StopDialog({
  eventId,
  outcome,
  reason,
  onClose,
  onConfirm,
}: {
  eventId: string;
  outcome: Outcome;
  reason: string;
  onClose: () => void;
  onConfirm: (reason: string) => Promise<void>;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [validation, setValidation] = useState("");
  useEffect(() => {
    dialog.current?.showModal();
  }, []);
  const locked = outcome.state !== "idle";
  const pending = outcome.state === "pending";
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    if (data.get("confirmation") !== "STOP") {
      setValidation("Type STOP exactly to confirm.");
      return;
    }
    try {
      setValidation("");
      await onConfirm(locked ? reason : String(data.get("reason") ?? ""));
    } catch (error) {
      setValidation(error instanceof Error ? error.message : "Invalid request");
    }
  }
  return (
    <dialog
      ref={dialog}
      className="confirmation"
      aria-labelledby="stop-title"
      onCancel={(event) => {
        if (pending) event.preventDefault();
        else onClose();
      }}
    >
      <form onSubmit={submit}>
        <p className="eyebrow">Explicit operator confirmation</p>
        <h2 id="stop-title">Request an emergency stop?</h2>
        <p>
          This asks the server to stop event{" "}
          <span className="mono">{eventId}</span>. A receipt does not mean the
          fleet has stopped.
        </p>
        <label>
          Reason for stopping
          <input
            name="reason"
            defaultValue={reason}
            required
            maxLength={1000}
            disabled={locked}
          />
        </label>
        <label>
          Type STOP to confirm
          <input
            name="confirmation"
            autoComplete="off"
            required
            disabled={pending}
          />
        </label>
        {validation && <p role="alert">{validation}</p>}
        {outcome.state === "unknown" && (
          <p role="alert">
            Outcome unknown. {outcome.error} Retry the same request to avoid
            duplicate intent.
          </p>
        )}
        {outcome.state === "blocked" && (
          <p role="alert">No stop command was sent. {outcome.error}</p>
        )}
        <div className="dialog-actions">
          <button
            type="button"
            className="secondary-button"
            disabled={pending}
            onClick={onClose}
          >
            Close
          </button>
          <button className="action-button stop-button" disabled={pending}>
            {pending
              ? "Awaiting stop receipt…"
              : outcome.state === "unknown"
                ? "Retry same request"
                : outcome.state === "blocked"
                  ? "Retry authorization"
                  : "Confirm stop request"}
          </button>
        </div>
      </form>
    </dialog>
  );
}

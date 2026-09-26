import { useEffect, useRef, useState, type FormEvent } from "react";
import type { Role } from "../api/client";
import {
  DispatchEventState,
  type DispatchEvent,
} from "../api/gen/gridos/v1/dispatch_pb";

export function ApprovalActions({
  event,
  role,
  onConfirm,
}: {
  event: DispatchEvent;
  role: Role;
  onConfirm: (action: "approve" | "launch") => Promise<void>;
}) {
  const [review, setReview] = useState(false);
  const action =
    event.state === DispatchEventState.VALIDATED
      ? "approve"
      : event.state === DispatchEventState.APPROVED
        ? "launch"
        : null;
  if (!action) return null;
  if (role !== "approver")
    return (
      <p className="boundary-note">
        Approver role required to approve or launch this plan.
      </p>
    );
  return (
    <div className="approval-actions">
      <button className="action-button" onClick={() => setReview(true)}>
        {action === "approve" ? "Review approval" : "Review launch"}
      </button>
      {review && (
        <Confirmation
          key={`${event.planVersion}:${action}`}
          action={action}
          version={event.planVersion.toString()}
          onCancel={() => setReview(false)}
          onConfirm={async () => {
            await onConfirm(action);
            setReview(false);
          }}
        />
      )}
    </div>
  );
}

function Confirmation({
  action,
  version,
  onConfirm,
  onCancel,
}: {
  action: "approve" | "launch";
  version: string;
  onConfirm: () => Promise<void>;
  onCancel: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    dialog.current?.showModal();
  }, []);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const entered = new FormData(event.currentTarget).get("version");
    if (entered !== version) {
      setError("Enter the exact plan version");
      return;
    }
    setPending(true);
    try {
      await onConfirm();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "Action failed");
      setPending(false);
    }
  }
  return (
    <dialog
      ref={dialog}
      className="confirmation"
      onCancel={(event) => {
        if (pending) event.preventDefault();
        else onCancel();
      }}
      aria-labelledby="confirm-title"
    >
      <form onSubmit={submit}>
        <p className="eyebrow">Explicit operator confirmation</p>
        <h2 id="confirm-title">
          {action === "approve"
            ? "Approve this plan?"
            : "Launch approved commands?"}
        </h2>
        <p>
          {action === "approve"
            ? "Approval records your authorization. It does not dispatch the fleet."
            : "Launch asks the server to persist and send commands for this approved plan. Receipt and delivery remain separate facts."}
        </p>
        <label>
          Type plan version {version}
          <input name="version" autoComplete="off" required />
        </label>
        {error && <p role="alert">{error}</p>}
        <div className="dialog-actions">
          <button
            type="button"
            className="secondary-button"
            disabled={pending}
            onClick={onCancel}
          >
            Cancel
          </button>
          <button className="action-button" disabled={pending}>
            {pending
              ? "Awaiting server…"
              : action === "approve"
                ? "Confirm approval"
                : "Confirm launch"}
          </button>
        </div>
      </form>
    </dialog>
  );
}

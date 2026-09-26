import { useState, type FormEvent } from "react";
import { z } from "zod";

export const dispatchSchema = z
  .object({
    region: z.literal("LZ_AEN"),
    begin: z.iso.datetime({ local: true, precision: -1 }),
    end: z.iso.datetime({ local: true, precision: -1 }),
    targetMw: z.coerce.number().positive("Enter a target greater than zero"),
    boundary: z.enum([
      "METER_NET_EXPORT",
      "BATTERY_TERMINAL",
      "IMPORT_REDUCTION_VS_BASELINE",
    ]),
  })
  .refine((value) => new Date(`${value.end}Z`) > new Date(`${value.begin}Z`), {
    message: "End must be after start",
    path: ["end"],
  });

export type DispatchInput = z.infer<typeof dispatchSchema>;

export function DispatchForm({
  onSubmit,
}: {
  onSubmit: (input: DispatchInput) => Promise<void>;
}) {
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const result = dispatchSchema.safeParse(
      Object.fromEntries(new FormData(event.currentTarget)),
    );
    if (!result.success) {
      setError(result.error.issues.map((issue) => issue.message).join(". "));
      return;
    }
    setPending(true);
    setError("");
    try {
      await onSubmit(result.data);
    } catch (failure) {
      setError(
        failure instanceof Error ? failure.message : "Dispatch request failed",
      );
    } finally {
      setPending(false);
    }
  }
  return (
    <form className="dispatch-form" onSubmit={submit}>
      <div className="section-heading">
        <div>
          <p className="eyebrow">Define the commitment</p>
          <h2>Request a dispatch plan</h2>
        </div>
        <span className="mode-chip">SIMULATED</span>
      </div>
      <p>
        The safety gate checks a proposed plan before approval. Creating a
        request does not send commands.
      </p>
      <label>
        Operating region
        <select name="region">
          <option value="LZ_AEN">Greater Austin · LZ_AEN</option>
        </select>
      </label>
      <div className="form-pair">
        <label>
          Start time
          <input name="begin" type="datetime-local" required />
        </label>
        <label>
          End time
          <input name="end" type="datetime-local" required />
        </label>
      </div>
      <p className="form-hint">
        All times are UTC. The event expires at the end of this window.
      </p>
      <div className="form-pair">
        <label>
          Target power (MW)
          <input
            name="targetMw"
            type="number"
            min="0.001"
            step="0.001"
            placeholder="5.000"
            required
          />
        </label>
        <label>
          Measurement boundary
          <select name="boundary">
            <option value="METER_NET_EXPORT">Meter net export</option>
            <option value="BATTERY_TERMINAL">Battery terminal</option>
            <option value="IMPORT_REDUCTION_VS_BASELINE">
              Import reduction vs baseline
            </option>
          </select>
        </label>
      </div>
      <div className="boundary-note">
        A meter export target is measured after household consumption. Battery
        discharge and meter export are different quantities.
      </div>
      {error && (
        <p role="alert" className="error-notice">
          {error}
        </p>
      )}
      <button className="action-button" disabled={pending}>
        {pending ? "Planning and validating…" : "Create dispatch plan"}
        <span aria-hidden="true">↗</span>
      </button>
    </form>
  );
}

import { useRef, useState } from "react";
import { clone, create } from "@bufbuild/protobuf";
import { z } from "zod";
import { useSession } from "../api/auth";
import { evidenceSchema } from "../api/Provenance";
import type { GetPlanExplanationResponse } from "../api/gen/gridos/v1/api_pb";
import { DispatchPlanSchema } from "../api/gen/gridos/v1/optimization_pb";

const schedulesSchema = z
  .array(
    z.object({
      deviceId: z.string().min(1),
      intervals: z
        .array(
          z.object({
            beginTime: evidenceSchema.shape.timestamp,
            endTime: evidenceSchema.shape.timestamp,
            setpointKw: z.number().finite(),
          }),
        )
        .min(1),
    }),
  )
  .min(1);
const responseSchema = z
  .object({
    approved: z.boolean(),
    violations: z.array(z.object({ code: z.string().min(1) })),
    operatorExplanation: z.string(),
  })
  .refine((value) => value.approved === (value.violations.length === 0));
type Outcome =
  | { state: "idle" }
  | { state: "pending" }
  | { state: "error"; message: string }
  | { state: "received"; result: z.infer<typeof responseSchema> };

export function UnsafeAlternative({
  eventId,
  planVersion,
  state,
  explanation,
}: {
  eventId: string;
  planVersion: bigint;
  state: number;
  explanation: GetPlanExplanationResponse;
}) {
  const { client, identity } = useSession();
  const [outcome, setOutcome] = useState<Outcome>({ state: "idle" });
  const pending = useRef(false);
  const schedules = explanation.evidence?.deviceSchedules;
  const available = schedulesSchema.safeParse(schedules).success;
  const authorized = ["operator", "approver"].includes(identity.role);
  async function validate() {
    if (!authorized || !available || state !== 3 || pending.current) return;
    pending.current = true;
    setOutcome({ state: "pending" });
    const alternativePlan = clone(
      DispatchPlanSchema,
      create(DispatchPlanSchema, {
        eventId,
        planVersion,
        deviceSchedules: schedules,
        shortfalls: explanation.shortfalls,
      }),
    );
    alternativePlan.deviceSchedules[0]!.intervals[0]!.setpointKw = 1000000;
    try {
      const response = await client.dispatch.validateUnsafeAlternative({
        eventId,
        planVersion,
        alternativePlan,
      });
      const result = responseSchema.safeParse(response);
      if (!result.success)
        throw new Error("Invalid safety validation response");
      setOutcome({ state: "received", result: result.data });
    } catch (error) {
      setOutcome({
        state: "error",
        message:
          error instanceof Error
            ? error.message
            : "Safety validation unavailable",
      });
    } finally {
      pending.current = false;
    }
  }
  return (
    <section
      className="plan-explanation"
      aria-label="Unsafe alternative validation"
    >
      <h3>Independent safety check</h3>
      {available ? (
        <p>
          Push one device to 1,000,000 kW and confirm the gate rejects it.
          Nothing is sent.
        </p>
      ) : (
        <p>
          Stored schedule evidence unavailable. An alternative cannot be
          constructed.
        </p>
      )}
      {state !== 3 && (
        <p>A safety-validated event awaiting approval is required.</p>
      )}
      {!authorized && <p>Operator or approver role required.</p>}
      <button
        className="secondary-button"
        disabled={
          !authorized ||
          !available ||
          state !== 3 ||
          outcome.state === "pending"
        }
        onClick={validate}
      >
        Validate unsafe alternative
      </button>
      {outcome.state === "pending" && (
        <p role="status">Checking against frozen device limits…</p>
      )}
      {outcome.state === "error" && <p role="alert">{outcome.message}</p>}
      {outcome.state === "received" && (
        <div role="status">
          <h4>
            {outcome.result.approved
              ? "Alternative passed validation"
              : "Alternative rejected"}
          </h4>
          <p>{outcome.result.operatorExplanation}</p>
          <ul>
            {outcome.result.violations.map((violation, index) => (
              <li key={index}>{violation.code}</li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}

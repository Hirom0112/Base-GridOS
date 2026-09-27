import { Code, ConnectError, type Interceptor } from "@connectrpc/connect";
import { z } from "zod";
import type { Identity } from "./client";

const approvalSchema = z.object({
  eventId: z.string().min(1),
  planVersion: z.bigint().positive().max(BigInt(Number.MAX_SAFE_INTEGER)),
});
const stopSchema = z.object({ eventId: z.string().min(1) });
const assertionSchema = z.object({ assertion: z.string().trim().min(1) });

export class StepUpFailure extends ConnectError {
  constructor(message: string, code = Code.Unavailable) {
    super(message, code);
  }
}

export function stepUpAuthorization(
  identity: Identity,
  fetcher: typeof fetch,
): Interceptor {
  return (next) => async (request) => {
    const approval =
      request.method.parent.typeName === "gridos.v1.DispatchService" &&
      request.method.name === "ApproveEvent";
    const stop =
      request.method.parent.typeName === "gridos.v1.EventsService" &&
      request.method.name === "EmergencyStop";
    if (!approval && !stop) return next(request);
    if (identity.mode !== "local")
      throw new StepUpFailure(
        "Identity-provider step-up authorization is not configured.",
        Code.FailedPrecondition,
      );
    try {
      const input = approval
        ? approvalSchema.parse(request.message)
        : { ...stopSchema.parse(request.message), planVersion: 0n };
      const response = await fetcher("/local/step-up", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-GridOS-Role": identity.role,
        },
        body: JSON.stringify({
          action: approval ? "APPROVE_EVENT" : "EMERGENCY_STOP",
          event_id: input.eventId,
          plan_version: Number(input.planVersion),
        }),
        signal: AbortSignal.any([request.signal, AbortSignal.timeout(10000)]),
        cache: "no-store",
        redirect: "error",
      });
      if (!response.ok)
        throw new StepUpFailure(
          response.status === 403
            ? "Step-up authorization denied."
            : "Step-up authorization service unavailable.",
          response.status === 403 ? Code.PermissionDenied : Code.Unavailable,
        );
      const { assertion } = assertionSchema.parse(await response.json());
      request.header.set("X-GridOS-Step-Up", assertion);
    } catch (error) {
      if (error instanceof StepUpFailure) throw error;
      throw new StepUpFailure(
        "Unable to obtain a valid step-up assertion. No command was sent.",
      );
    }
    return next(request);
  };
}

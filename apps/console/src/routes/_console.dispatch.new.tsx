import { useRef } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { useSession } from "../api/auth";
import { MeasurementBoundary } from "../api/gen/gridos/v1/telemetry_pb";
import { DispatchForm, type DispatchInput } from "../dispatch/dispatch-form";

export const Route = createFileRoute("/_console/dispatch/new")({
  component: NewDispatch,
});

function NewDispatch() {
  const { client, identity } = useSession();
  const navigate = useNavigate();
  const intent = useRef({ fingerprint: "", key: "" });
  async function submit(input: DispatchInput) {
    const fingerprint = JSON.stringify(input);
    if (intent.current.fingerprint !== fingerprint)
      intent.current = { fingerprint, key: crypto.randomUUID() };
    const response = await client.dispatch.createEventRequest({
      idempotencyKey: intent.current.key,
      eventRequest: {
        requestId: intent.current.key,
        correlationId: intent.current.key,
        eventType: "ERCOT_DEMAND_RESPONSE",
        loadZones: [input.region],
        targetKw: input.targetMw * 1000,
        beginTime: timestampFromDate(new Date(`${input.begin}Z`)),
        endTime: timestampFromDate(new Date(`${input.end}Z`)),
        measurementBoundary: MeasurementBoundary[input.boundary],
      },
    });
    if (!response.event?.eventId)
      throw new Error("Server returned no event identifier");
    await navigate({
      to: "/dispatch/$eventId",
      params: { eventId: response.event.eventId },
    });
  }
  if (identity.role !== "operator")
    return (
      <div className="boundary-note">
        Switch to the operator role to create a dispatch request.
      </div>
    );
  return <DispatchForm onSubmit={submit} />;
}

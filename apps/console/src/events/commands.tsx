import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { useSession } from "../api/auth";
import { evidenceSchema } from "../api/Provenance";
import type { ListEventCommandsResponse } from "../api/gen/gridos/v1/events_pb";
import "./report.css";

const timeSchema = evidenceSchema.shape.timestamp;
const commandSchema = z
  .object({
    intent: z.object({
      commandId: z.string().min(1),
      eventId: z.string().min(1),
      deviceId: z.string().min(1),
      planVersion: z.bigint().positive(),
      generation: z.bigint().nonnegative(),
      setpointKw: z.number().finite(),
      issuedAt: timeSchema,
      effectiveAt: timeSchema,
      expiresAt: timeSchema,
    }),
    state: z.string().min(1),
    stateRecordedAt: timeSchema,
    receipt: z
      .object({
        commandId: z.string().min(1),
        acknowledgementId: z.string().min(1),
        receiptStatus: z.union([z.literal(1), z.literal(2)]),
        gatewayId: z.string().min(1),
        receivedAt: timeSchema,
        rejectionReason: z.string(),
      })
      .optional(),
  })
  .refine(
    (command) =>
      !command.receipt ||
      command.receipt.commandId === command.intent.commandId,
  )
  .refine(
    ({ intent }) =>
      intent.expiresAt.seconds > intent.effectiveAt.seconds ||
      (intent.expiresAt.seconds === intent.effectiveAt.seconds &&
        intent.expiresAt.nanos > intent.effectiveAt.nanos),
  );
const intervalSchema = z
  .object({
    beginTime: timeSchema,
    endTime: timeSchema,
    requestedKw: z.number().finite(),
    commandedKw: z.number().finite(),
    deliveredKw: z.number().finite(),
    trackingErrorKw: z.number().finite(),
    confidence: z.number().min(0).max(1),
    measurementBoundary: z.string().min(1),
    baselineMethod: z.string().min(1),
    valueKind: z.literal("MEASURED"),
  })
  .refine(
    (row) =>
      row.endTime.seconds > row.beginTime.seconds ||
      (row.endTime.seconds === row.beginTime.seconds &&
        row.endTime.nanos > row.beginTime.nanos),
  );
const commandsSchema = z
  .object({
    commands: z.array(commandSchema),
    verificationIntervals: z.array(intervalSchema),
  })
  .refine(
    ({ commands }) =>
      new Set(commands.map((row) => row.intent.commandId)).size ===
      commands.length,
  );

export function EventCommands({ eventId }: { eventId: string }) {
  const { client, identity } = useSession();
  const allowed = ["operator", "approver", "analyst", "service"].includes(
    identity.role,
  );
  const query = useQuery({
    queryKey: ["event-commands", eventId, identity.role],
    queryFn: ({ signal }) =>
      client.events.listEventCommands({ eventId }, { signal }),
    refetchInterval: 3000,
    enabled: allowed,
  });
  if (!allowed) return <p>Command evidence requires an operational role.</p>;
  if (query.isPending)
    return <p role="status">Loading durable command evidence…</p>;
  if (query.isError)
    return (
      <p role="alert">
        Command evidence unavailable.{" "}
        <button onClick={() => query.refetch()}>Retry commands</button>
      </p>
    );
  return <CommandEvidence eventId={eventId} data={query.data} />;
}

export function CommandEvidence({
  eventId,
  data,
}: {
  eventId: string;
  data: ListEventCommandsResponse;
}) {
  const [limit, setLimit] = useState(50);
  const parsed = commandsSchema.safeParse(data);
  if (
    !parsed.success ||
    parsed.data.commands.some((row) => row.intent.eventId !== eventId)
  )
    return (
      <p role="alert">
        Command evidence is invalid. Receipt and delivery cannot be established.
      </p>
    );
  const { commands, verificationIntervals } = parsed.data;
  const zero = commands.filter((row) => row.intent.setpointKw === 0);
  return (
    <div className="event-report">
      <section aria-label="Command fan-out">
        <h3>Durable command records</h3>
        <p>
          {commands.length} intents recorded. Latest{" "}
          {Math.min(limit, commands.length)} shown. Acknowledgement establishes
          receipt only.
        </p>
        {commands.length ? (
          <>
            <div
              className="report-scroll"
              role="region"
              aria-label="Command records table"
              tabIndex={0}
            >
              <table aria-label="Command records">
                <thead>
                  <tr>
                    <th>Command and device</th>
                    <th>Intent</th>
                    <th>Durable state</th>
                    <th>Receipt</th>
                    <th>Effective and expiry (UTC)</th>
                  </tr>
                </thead>
                <tbody>
                  {commands
                    .slice(-limit)
                    .reverse()
                    .map((row) => (
                      <tr key={row.intent.commandId}>
                        <th scope="row">
                          {row.intent.commandId}
                          <br />
                          {row.intent.deviceId}
                          <br />
                          Plan {row.intent.planVersion.toString()} · generation{" "}
                          {row.intent.generation.toString()}
                        </th>
                        <td>
                          {row.intent.setpointKw} kW
                          <br />
                          Issued <CommandTime value={row.intent.issuedAt} />
                        </td>
                        <td>
                          {row.state}
                          <br />
                          <CommandTime value={row.stateRecordedAt} />
                        </td>
                        <td>
                          {row.receipt ? (
                            <>
                              {row.receipt.receiptStatus === 1
                                ? "ACCEPTED"
                                : "REJECTED"}
                              <br />
                              {row.receipt.gatewayId}
                              <br />
                              <CommandTime value={row.receipt.receivedAt} />
                              {row.receipt.rejectionReason && (
                                <p>{row.receipt.rejectionReason}</p>
                              )}
                            </>
                          ) : (
                            "No receipt returned"
                          )}
                        </td>
                        <td>
                          <CommandTime value={row.intent.effectiveAt} />
                          <br />
                          <CommandTime value={row.intent.expiresAt} />
                        </td>
                      </tr>
                    ))}
                </tbody>
              </table>
            </div>
            {limit < commands.length && (
              <button onClick={() => setLimit(limit + 50)}>
                Show 50 earlier commands
              </button>
            )}
          </>
        ) : (
          <p>No command intents returned.</p>
        )}
      </section>
      <section aria-label="Safe return evidence">
        <h3>Expiry and zero-setpoint evidence</h3>
        <p>
          Expiry is the command’s validity limit. Zero-setpoint intent and
          receipt do not confirm the fleet has stopped.
        </p>
        {zero.length ? (
          <p>
            {zero.length} zero-setpoint intents ·{" "}
            {zero.filter((row) => row.receipt?.receiptStatus === 1).length}{" "}
            accepted receipts. Inspect their recorded state and expiry in the
            command table.
          </p>
        ) : (
          <p>No zero-setpoint intents returned.</p>
        )}
      </section>
      <IntervalVerification rows={verificationIntervals} />
    </div>
  );
}

function IntervalVerification({
  rows,
}: {
  rows: z.infer<typeof intervalSchema>[];
}) {
  return (
    <section aria-label="Measured interval verification">
      <h3>Measured interval verification</h3>
      <p>
        Verification applies to the event’s measurement boundary and interval,
        not individual command receipts.
      </p>
      {rows.length ? (
        <div
          className="report-scroll"
          role="region"
          aria-label="Verification intervals table"
          tabIndex={0}
        >
          <table aria-label="Verification intervals">
            <thead>
              <tr>
                <th>Window (UTC)</th>
                <th>Requested / commanded</th>
                <th>Measured delivery</th>
                <th>Tracking error</th>
                <th>Method</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row, index) => (
                <tr key={index}>
                  <td>
                    <CommandTime value={row.beginTime} />
                    <br />
                    <CommandTime value={row.endTime} />
                  </td>
                  <td>
                    {row.requestedKw} / {row.commandedKw} kW
                  </td>
                  <td>{row.deliveredKw} kW · MEASURED</td>
                  <td>{row.trackingErrorKw} kW</td>
                  <td>
                    {row.measurementBoundary}
                    <br />
                    {row.baselineMethod}
                    <br />
                    {row.confidence * 100}% confidence
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <p>
          No measured verification intervals returned. Delivery remains
          unavailable.
        </p>
      )}
    </section>
  );
}

function CommandTime({ value }: { value: z.infer<typeof timeSchema> }) {
  const iso = new Date(
    Number(value.seconds) * 1000 + value.nanos / 1e6,
  ).toISOString();
  return <time dateTime={iso}>{iso}</time>;
}

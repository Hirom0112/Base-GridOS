import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { z } from "zod";
import { useSession } from "../api/auth";
import type { ReplayEventResponse } from "../api/gen/gridos/v1/api_pb";
import { AuditRecords, recordsSchema } from "./audit-timeline";
import { useReplayClock } from "./replay-clock";
import { ReplayDelivery } from "./replay-delivery";

const replaySchema = z
  .object({
    seed: z.bigint(),
    inputSnapshotId: z.string().min(1),
    eligibilitySnapshotId: z.string().min(1),
    policyVersion: z.string().min(1),
    solverVersion: z.string().min(1),
    fallbackVersion: z.string(),
    codeVersion: z.string().min(1),
    fleetSha256: z.string().regex(/^[a-f0-9]{64}$/),
    scenarioSha256: z.union([
      z.literal(""),
      z.string().regex(/^[a-f0-9]{64}$/),
    ]),
    updates: recordsSchema.refine((records) =>
      records.every((record, index) => {
        if (index === 0) return true;
        const previous = records[index - 1]!.occurredAt;
        return (
          record.occurredAt.seconds * 1000000000n +
            BigInt(record.occurredAt.nanos) >=
          previous.seconds * 1000000000n + BigInt(previous.nanos)
        );
      }),
    ),
    diffStatus: z.enum(["IDENTICAL", "DIFFERENT"]),
    differences: z.array(
      z.object({
        field: z.string().min(1),
        expectedJson: z.string(),
        actualJson: z.string(),
      }),
    ),
  })
  .refine(
    (result) =>
      (result.diffStatus === "IDENTICAL") === (result.differences.length === 0),
  );

export function ReplayEvidence({
  data,
  eventId,
}: {
  data: ReplayEventResponse;
  eventId: string;
}) {
  const { position: clock, setPosition } = useReplayClock();
  const position =
    clock?.eventId === eventId
      ? Math.min(clock.index, data.updates.length - 1)
      : 0;
  const result = replaySchema.safeParse(data);
  const first = result.success ? data.updates[0]?.occurredAt : undefined;
  useEffect(() => {
    if (first)
      setPosition((current) =>
        current?.eventId === eventId
          ? current
          : { eventId, index: 0, at: first },
      );
  }, [first, eventId, setPosition]);
  if (!result.success)
    return (
      <p role="alert">
        Replay evidence is invalid. No replay can be established.
      </p>
    );
  const current = data.updates[Math.min(position, data.updates.length - 1)];
  const time = current?.occurredAt
    ? timestampDate(current.occurredAt).toISOString()
    : "No replay updates supplied";
  return (
    <div className="event-report">
      <p>
        Recomputed plan comparison: <strong>{data.diffStatus}</strong>
      </p>
      <p>
        Deterministic planning comparison; this result does not verify physical
        delivery.
      </p>
      <dl className="report-values">
        {[
          ["Seed", data.seed.toString()],
          ["Input snapshot", data.inputSnapshotId],
          ["Eligibility snapshot", data.eligibilitySnapshotId],
          ["Policy", data.policyVersion],
          ["Solver", data.solverVersion],
          ["Fallback", data.fallbackVersion || "Not supplied"],
          ["Code", data.codeVersion],
          ["Fleet SHA-256", data.fleetSha256],
          ["Scenario SHA-256", data.scenarioSha256 || "Not supplied"],
        ].map(([name, value]) => (
          <div key={name}>
            <dt>{name}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
      {data.differences.length > 0 && (
        <section aria-label="Replay differences">
          <h4>Differences</h4>
          {data.differences.map((difference, index) => (
            <div key={index}>
              <strong>{difference.field}</strong>
              <p>
                Expected: <code>{difference.expectedJson}</code>
              </p>
              <p>
                Actual: <code>{difference.actualJson}</code>
              </p>
            </div>
          ))}
        </section>
      )}
      <p aria-label="Replay time">{time}</p>
      {data.updates.length > 0 && (
        <>
          <label>
            Replay position
            <input
              type="range"
              min={0}
              max={data.updates.length - 1}
              step={1}
              value={position}
              onChange={(event) => {
                const index = z.coerce
                  .number()
                  .int()
                  .min(0)
                  .max(data.updates.length - 1)
                  .parse(event.target.value);
                setPosition({
                  eventId,
                  index,
                  at: data.updates[index]!.occurredAt!,
                });
              }}
            />
          </label>
          <AuditRecords entries={data.updates.slice(0, position + 1)} />
        </>
      )}
      <p className="boundary-note">
        The replay timestamp controls historical geography. Per-cell command and
        measured response histories are unavailable; no delivery is inferred
        from capacity.
      </p>
    </div>
  );
}

export function EventReplay({ eventId }: { eventId: string }) {
  const { position } = useReplayClock();
  const { client, identity } = useSession();
  const [requested, setRequested] = useState(false);
  const allowed = ["operator", "approver", "analyst", "service"].includes(
    identity.role,
  );
  const query = useQuery({
    queryKey: ["event-replay", eventId, identity.role],
    queryFn: ({ signal }) =>
      client.replay.replayEvent({ eventId }, { signal, timeoutMs: 25000 }),
    enabled: allowed && requested,
    retry: false,
  });
  if (!allowed) return null;
  return (
    <section className="event-report" aria-label="Event replay">
      <h3>Reproduce the recorded plan</h3>
      <button
        className="secondary-button"
        disabled={query.isFetching}
        onClick={() => (requested ? void query.refetch() : setRequested(true))}
      >
        Replay event
      </button>
      {query.isFetching && (
        <p role="status">Recomputing from recorded inputs…</p>
      )}
      {query.isError && (
        <p role="alert">Replay unavailable: {query.error.message}</p>
      )}
      {query.data && (
        <ReplayEvidence
          key={query.dataUpdatedAt}
          data={query.data}
          eventId={eventId}
        />
      )}
      {query.data && position?.eventId === eventId && (
        <ReplayDelivery eventId={eventId} asOf={position.at} />
      )}
    </section>
  );
}

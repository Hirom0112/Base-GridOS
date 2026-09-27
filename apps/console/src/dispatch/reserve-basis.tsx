import { useState } from "react";
import { z } from "zod";
import { evidenceSchema, provenanceNames } from "../api/Provenance";
import type { PlanExplanationEvidence } from "../api/gen/gridos/v1/optimization_pb";

const provenance = z.union([
  z.literal(1),
  z.literal(2),
  z.literal(3),
  z.literal(4),
  z.literal(5),
]);
const reserveSchema = z
  .object({
    deviceId: z.string().min(1),
    hardwareFloorKwh: z.number().nonnegative(),
    planReserveKwh: z.number().nonnegative(),
    policyFloorKwh: z.number().nonnegative(),
    effectiveReserveKwh: z.number().nonnegative(),
    policyVersion: z.string().min(1),
    overrideFloorKwh: z.number().nonnegative().optional(),
    overrideReason: z.number().int().min(0).max(7),
    overrideSourceId: z.string(),
    overridePolicyVersion: z.string(),
    provenance,
    issuedAt: evidenceSchema.shape.timestamp,
  })
  .refine((basis) =>
    basis.overrideFloorKwh === undefined
      ? basis.overrideReason === 0 &&
        basis.overrideSourceId === "" &&
        basis.overridePolicyVersion === ""
      : basis.overrideReason > 0 &&
        basis.overrideSourceId.length > 0 &&
        basis.overridePolicyVersion.length > 0,
  );
const travelSchema = z
  .object({
    windowId: z.string().min(1),
    memberId: z.string().min(1),
    siteId: z.string().min(1),
    startTime: evidenceSchema.shape.timestamp,
    endTime: evidenceSchema.shape.timestamp,
    creditType: z.union([z.literal(1), z.literal(2), z.literal(3)]),
    creditCents: z.bigint().nonnegative(),
    consentVersion: z.string().min(1),
    policyVersion: z.string().min(1),
    provenance,
    issuedAt: evidenceSchema.shape.timestamp,
  })
  .refine(
    ({ startTime, endTime }) =>
      endTime.seconds > startTime.seconds ||
      (endTime.seconds === startTime.seconds &&
        endTime.nanos > startTime.nanos),
  );
const overrideNames = [
  "UNSPECIFIED",
  "WEATHER",
  "OUTAGE_RISK",
  "HEALTH",
  "STALE_TELEMETRY",
  "ALARM",
  "EARLY_RETURN",
  "COMMUNICATIONS",
];
const creditNames = {
  1: "Fixed daily credit",
  2: "Fixed event credit",
  3: "Fixed annual credit",
};
const energy = (value: number) => `${value.toFixed(3)} kWh`;
const iso = (value: z.infer<typeof evidenceSchema.shape.timestamp>) =>
  new Date(Number(value.seconds) * 1000 + value.nanos / 1e6).toISOString();

export function HouseholdReserveBasis({
  evidence,
}: {
  evidence: PlanExplanationEvidence | undefined;
}) {
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState("");
  const parsed = z
    .array(reserveSchema)
    .max(100000)
    .safeParse(evidence?.reserveBases ?? []);
  const matches = parsed.success
    ? parsed.data.filter((basis) =>
        basis.deviceId.toLowerCase().includes(search.trim().toLowerCase()),
      )
    : [];
  const choices = matches.slice(0, 50);
  const basis = choices.find((row) => row.deviceId === selected) ?? choices[0];
  return (
    <section aria-label="Household reserve basis">
      <h3>Household reserve basis</h3>
      <p>
        Frozen at plan creation. The selected plan’s effective reserve is shown
        as served; independent safety validation remains required.
      </p>
      {!parsed.success ? (
        <p role="alert">Frozen reserve basis is invalid.</p>
      ) : !parsed.data.length ? (
        <p>Frozen reserve basis unavailable.</p>
      ) : (
        <>
          <p>
            {parsed.data.length.toLocaleString()} devices ·{" "}
            {parsed.data
              .filter((row) => row.overrideFloorKwh !== undefined)
              .length.toLocaleString()}{" "}
            active overrides in the frozen evidence.
          </p>
          <label className="forecast-filter">
            Find reserve device
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <p>
            Showing {choices.length} of {matches.length.toLocaleString()}{" "}
            matching devices.
          </p>
          {basis ? (
            <>
              <label className="forecast-filter">
                Reserve device
                <select
                  value={basis.deviceId}
                  onChange={(event) => setSelected(event.target.value)}
                >
                  {choices.map((row) => (
                    <option key={row.deviceId}>{row.deviceId}</option>
                  ))}
                </select>
              </label>
              <ReserveDetails basis={basis} />
            </>
          ) : (
            <p>No matching reserve devices.</p>
          )}
        </>
      )}
    </section>
  );
}

function ReserveDetails({ basis }: { basis: z.infer<typeof reserveSchema> }) {
  return (
    <div className="frozen-policy-details">
      <dl>
        <div>
          <dt>Hardware floor</dt>
          <dd>{energy(basis.hardwareFloorKwh)}</dd>
        </div>
        <div>
          <dt>Plan reserve</dt>
          <dd>{energy(basis.planReserveKwh)}</dd>
        </div>
        <div>
          <dt>Policy floor</dt>
          <dd>{energy(basis.policyFloorKwh)}</dd>
        </div>
        <div>
          <dt>Effective reserve</dt>
          <dd>{energy(basis.effectiveReserveKwh)}</dd>
        </div>
        <div>
          <dt>Policy version</dt>
          <dd>{basis.policyVersion}</dd>
        </div>
      </dl>
      {basis.overrideFloorKwh === undefined ? (
        <p>No active override in the frozen evidence.</p>
      ) : (
        <dl>
          <div>
            <dt>Override floor</dt>
            <dd>{energy(basis.overrideFloorKwh)}</dd>
          </div>
          <div>
            <dt>Override reason</dt>
            <dd>{overrideNames[basis.overrideReason]}</dd>
          </div>
          <div>
            <dt>Override source</dt>
            <dd>{basis.overrideSourceId}</dd>
          </div>
          <div>
            <dt>Override policy</dt>
            <dd>{basis.overridePolicyVersion}</dd>
          </div>
        </dl>
      )}
      <p>
        {provenanceNames[basis.provenance]} · Frozen{" "}
        <time dateTime={iso(basis.issuedAt)}>{iso(basis.issuedAt)}</time>
      </p>
    </div>
  );
}

export function TravelFlexEvidence({
  evidence,
}: {
  evidence: PlanExplanationEvidence | undefined;
}) {
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState("");
  const parsed = z
    .array(travelSchema)
    .max(100000)
    .safeParse(evidence?.travelFlexBindings ?? []);
  const matches = parsed.success
    ? parsed.data.filter((row) =>
        [row.windowId, row.memberId, row.siteId].some((id) =>
          id.toLowerCase().includes(search.trim().toLowerCase()),
        ),
      )
    : [];
  const choices = matches.slice(0, 50);
  const binding =
    choices.find((row) => row.windowId === selected) ?? choices[0];
  return (
    <section aria-label="Travel Flex eligibility">
      <h3>Travel Flex eligibility</h3>
      <p>
        Consented windows in the frozen eligibility snapshot. A bound credit is
        not proof of selection, settlement or payment.
      </p>
      {!parsed.success ? (
        <p role="alert">Frozen Travel Flex binding is invalid.</p>
      ) : !parsed.data.length ? (
        <p>Frozen Travel Flex bindings unavailable.</p>
      ) : (
        <>
          <label className="forecast-filter">
            Find Travel Flex window
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <p>
            Showing {choices.length} of {matches.length.toLocaleString()}{" "}
            matching windows.
          </p>
          {binding ? (
            <>
              <label className="forecast-filter">
                Travel Flex window
                <select
                  value={binding.windowId}
                  onChange={(event) => setSelected(event.target.value)}
                >
                  {choices.map((row) => (
                    <option key={row.windowId}>{row.windowId}</option>
                  ))}
                </select>
              </label>
              <TravelDetails binding={binding} />
            </>
          ) : (
            <p>No matching Travel Flex windows.</p>
          )}
        </>
      )}
    </section>
  );
}

function TravelDetails({ binding }: { binding: z.infer<typeof travelSchema> }) {
  return (
    <div className="frozen-policy-details">
      <dl>
        <div>
          <dt>Member</dt>
          <dd>{binding.memberId}</dd>
        </div>
        <div>
          <dt>Site</dt>
          <dd>{binding.siteId}</dd>
        </div>
        <div>
          <dt>Window begins (UTC)</dt>
          <dd>
            <time dateTime={iso(binding.startTime)}>
              {iso(binding.startTime)}
            </time>
          </dd>
        </div>
        <div>
          <dt>Window ends (UTC)</dt>
          <dd>
            <time dateTime={iso(binding.endTime)}>{iso(binding.endTime)}</time>
          </dd>
        </div>
        <div>
          <dt>Bound credit</dt>
          <dd>
            {binding.creditCents.toString()} cents ·{" "}
            {creditNames[binding.creditType]}
          </dd>
        </div>
        <div>
          <dt>Stored consent</dt>
          <dd>{binding.consentVersion}</dd>
        </div>
        <div>
          <dt>Stored policy</dt>
          <dd>{binding.policyVersion}</dd>
        </div>
      </dl>
      <p>
        {provenanceNames[binding.provenance]} · Frozen{" "}
        <time dateTime={iso(binding.issuedAt)}>{iso(binding.issuedAt)}</time>
      </p>
    </div>
  );
}

import { useState, type ReactNode } from "react";
import { Link, useHydrated } from "@tanstack/react-router";

const stages = [
  ["Observe", "Understand the fleet"],
  ["Forecast", "Anticipate conditions"],
  ["Optimize", "Find a feasible plan"],
  ["Approve", "Review and authorize"],
  ["Dispatch", "Send approved intent"],
  ["Verify", "Measure the response"],
  ["Learn", "Replay and improve"],
] as const;

export function Shell({
  initialTheme = "dark",
  children,
  evidence,
  observedAt,
  identity,
  eventId,
}: {
  initialTheme?: "dark" | "light";
  children?: ReactNode;
  evidence?: ReactNode;
  observedAt?: string;
  identity?: string;
  eventId?: string;
}) {
  const [theme, setTheme] = useState(initialTheme);
  const hydrated = useHydrated();
  return (
    <div className="console" data-theme={theme}>
      <a className="skip-link" href="#fleet-overview">
        Skip to fleet overview
      </a>
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true">
            ▦
          </span>
          GridOS
        </div>
        <span className="scope">
          Greater Austin <span className="mono">/ LZ_AEN</span>
        </span>
        <span className="scenario mono">
          {observedAt ?? "Scenario time unavailable"}
        </span>
        <span className="mode-chip">SIMULATED</span>
        <button
          className="theme-toggle"
          disabled={!hydrated}
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
          aria-label={`Use ${theme === "dark" ? "light" : "dark"} theme`}
        >
          <span aria-hidden="true">◐</span>
          <span>{theme === "dark" ? "Light" : "Dark"}</span>
        </button>
        <span className="identity">{identity ?? "Session not connected"}</span>
      </header>
      <OperatingRail eventId={eventId} connected={Boolean(children)} />
      <main
        id="fleet-overview"
        className="workspace"
        aria-labelledby="fleet-title"
        tabIndex={-1}
      >
        {children ?? <FleetOverview />}
        {evidence ?? <EvidenceRail />}
      </main>
      <footer className="truth-strip">
        <strong>
          <span className="unavailable-dot" aria-hidden="true" />
          {observedAt
            ? "Fleet observation recorded"
            : "Fleet state unavailable"}
        </strong>
        <span>{eventId ?? "No event selected"}</span>
        <span className="mono">SIMULATED</span>
        <span>
          {observedAt ? "Freshness shown per aggregate" : "Freshness unknown"}
        </span>
        <span className="truth-tail">
          {observedAt
            ? "Acknowledgement is not delivery"
            : "Awaiting server evidence"}
        </span>
      </footer>
    </div>
  );
}

function OperatingRail({
  eventId,
  connected,
}: {
  eventId?: string;
  connected: boolean;
}) {
  return (
    <nav className="operating-rail" aria-label="Operating loop">
      <p className="eyebrow">Operating loop</p>
      <p className="rail-intro">From insight to impact.</p>
      <ol>
        {stages.map(([name, description], index) => (
          <li key={name}>
            {connected ? (
              <StageLink
                index={index}
                eventId={eventId}
                name={name}
                description={description}
              />
            ) : index === 0 ? (
              <a
                className="stage selected"
                href="#fleet-overview"
                aria-current="page"
              >
                <span className="stage-number mono">01</span>
                <span>
                  <strong>{name}</strong>
                  <small>{description}</small>
                </span>
                <span className="stage-dot" aria-hidden="true" />
              </a>
            ) : (
              <span className="stage" aria-disabled="true">
                <span className="stage-number mono">0{index + 1}</span>
                <span>
                  <strong>{name}</strong>
                  <small>{description}</small>
                </span>
              </span>
            )}
          </li>
        ))}
      </ol>
      <div className="event-thread">
        <p className="eyebrow">Event thread</p>
        <p>{eventId ?? "No event selected"}</p>
        <small>A versioned event connects every stage of the loop.</small>
      </div>
      <div className="rail-footnote">
        <span className="brand-mark" aria-hidden="true">
          ▦
        </span>
        <span>
          Household resilience.
          <br />
          Grid-scale coordination.
        </span>
      </div>
    </nav>
  );
}

function StageLink({
  index,
  eventId,
  name,
  description,
}: {
  index: number;
  eventId?: string;
  name: string;
  description: string;
}) {
  const content = (
    <>
      <span className="stage-number mono">0{index + 1}</span>
      <span>
        <strong>{name}</strong>
        <small>{description}</small>
      </span>
    </>
  );
  if (index === 0)
    return (
      <Link
        to="/fleet"
        className="stage"
        activeProps={{ className: "stage selected" }}
      >
        {content}
      </Link>
    );
  if (index === 2)
    return (
      <Link
        to="/dispatch/new"
        className="stage"
        activeProps={{ className: "stage selected" }}
      >
        {content}
      </Link>
    );
  if (!eventId || index === 1)
    return (
      <span className="stage" aria-disabled="true">
        {content}
      </span>
    );
  if (index === 3)
    return (
      <Link
        to="/dispatch/$eventId"
        params={{ eventId }}
        className="stage"
        activeProps={{ className: "stage selected" }}
      >
        {content}
      </Link>
    );
  if (index < 6)
    return (
      <Link
        to="/events/$eventId"
        params={{ eventId }}
        className="stage"
        activeOptions={{ exact: true }}
        activeProps={index === 4 ? { className: "stage selected" } : {}}
      >
        {content}
      </Link>
    );
  return (
    <Link
      to="/events/$eventId/report"
      params={{ eventId }}
      className="stage"
      activeProps={{ className: "stage selected" }}
    >
      {content}
    </Link>
  );
}

function FleetOverview() {
  return (
    <div className="situational-field">
      <div className="view-heading">
        <div>
          <p className="eyebrow">Fleet / Observe</p>
          <h1 id="fleet-title">Austin fleet</h1>
          <p>One fleet. Every decision grounded in evidence.</p>
        </div>
        <div className="primary-action">
          <button disabled>
            Plan a dispatch <span aria-hidden="true">↗</span>
          </button>
          <small>Fleet data and authorization required</small>
        </div>
      </div>
      <section className="capacity-strip" aria-label="Fleet capacity">
        {[
          ["Installed power", "MW"],
          ["Usable energy", "MWh"],
          ["Reserved for backup", "MWh"],
        ].map(([label, unit]) => (
          <div className="capacity" key={label}>
            <span>{label}</span>
            <p>
              <span aria-label="Unavailable">—</span>
              <small className="mono">{unit}</small>
            </p>
            <small>Awaiting observation</small>
          </div>
        ))}
      </section>
      <section className="grid-field" aria-labelledby="grid-title">
        <div className="field-header">
          <span className="eyebrow">The Living Grid</span>
          <span className="mono">Greater Austin · H3 aggregates</span>
        </div>
        <div className="field-empty">
          <svg
            className="observation-symbol"
            viewBox="0 0 160 144"
            fill="none"
            aria-hidden="true"
          >
            <path d="M80 8 136 40V104L80 136 24 104V40Z" />
            <path d="M80 32 115 52V92L80 112 45 92V52Z" />
            <path d="M80 55 95 64V81L80 90 65 81V64Z" />
            <path d="M80 8V32M136 40 115 52M136 104 115 92M80 136V112M24 104 45 92M24 40 45 52" />
          </svg>
          <span className="empty-label mono">OBSERVATION PENDING</span>
          <h2 id="grid-title">Awaiting fleet observation</h2>
          <p>
            Capacity, availability, and reserve will appear here
            <br className="desktop-break" /> when a timestamped fleet response
            is available.
          </p>
          <span className="data-gap">No fleet observation received</span>
        </div>
        <div className="field-footer">
          <span>Aggregate geography only</span>
          <span>No operational state inferred</span>
        </div>
      </section>
      <section className="response-boundary" aria-labelledby="response-title">
        <div>
          <p className="eyebrow" id="response-title">
            From intent to evidence
          </p>
          <p>Every response has its own proof.</p>
        </div>
        <ol>
          <li>
            <span className="proof-line" />
            Sent<small>Command intent</small>
          </li>
          <li>
            <span className="proof-line dashed" />
            Acknowledged<small>Receipt confirmed</small>
          </li>
          <li>
            <span className="proof-line double" />
            Delivered<small>Telemetry verified</small>
          </li>
        </ol>
      </section>
    </div>
  );
}

function EvidenceRail() {
  return (
    <aside className="evidence-rail" aria-labelledby="evidence-title">
      <div className="evidence-heading">
        <h2 id="evidence-title">Fleet evidence</h2>
        <span className="mono">01 / 07</span>
      </div>
      <section className="evidence-card">
        <p className="eyebrow">Observation</p>
        <h3>No evidence yet</h3>
        <p>Fleet health cannot be established until an observation arrives.</p>
        <dl>
          <div>
            <dt>Observed at</dt>
            <dd>Unavailable</dd>
          </div>
          <div>
            <dt>Freshness</dt>
            <dd>Unknown</dd>
          </div>
          <div>
            <dt>Provenance mix</dt>
            <dd>Unavailable</dd>
          </div>
        </dl>
      </section>
      <section className="evidence-card">
        <p className="eyebrow">Environment</p>
        <span className="mode-chip">SIMULATED</span>
        <h3>A controlled demonstration</h3>
        <p>
          Generated fleet and scenario data. No production batteries are
          controlled.
        </p>
      </section>
      <section className="evidence-card reserve-card">
        <p className="eyebrow">Operating principle</p>
        <h3>Reserve comes first.</h3>
        <p>
          Household backup limits remain protected in every plan. Insufficient
          capacity is reported as a shortfall.
        </p>
      </section>
      <p className="evidence-note">
        The operator sees what is known,
        <br />
        when it was known, and why.
      </p>
    </aside>
  );
}

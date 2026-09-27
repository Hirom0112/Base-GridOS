import { z } from "zod";
import type { EventSample } from "../events/events-live";
import { useEffect, useMemo, useRef, useState } from "react";
import type { H3SiteAggregate } from "../api/gen/gridos/v1/api_pb";
import {
  projectCells,
  projectResponse,
  type GridCell,
  type ResponseMeasure,
} from "./scene";
import type { mountGrid } from "./renderer";
import { Evidence, provenanceNames } from "../api/Provenance";

export default function LivingGrid({
  cells,
  selected,
  onSelect,
  active = true,
  response,
}: {
  cells: H3SiteAggregate[];
  selected: string | null;
  onSelect: (id: string) => void;
  active?: boolean;
  response?: EventSample;
}) {
  const [measure, setMeasure] = useState<"installed" | ResponseMeasure>(
    "installed",
  );
  const projection = useMemo(() => {
    try {
      const projected = projectCells(cells);
      return {
        cells:
          measure === "installed"
            ? projected
            : projectResponse(
                projected,
                response ?? { state: 0, h3: [] },
                measure,
              ),
        error: null,
      };
    } catch {
      return {
        cells: [],
        error:
          "Geography cannot be displayed: invalid location or capacity evidence.",
      };
    }
  }, [cells, response, measure]);
  const sources =
    measure === "installed"
      ? (projection.error ? [] : cells).map(
          (cell) => cell.installedMw?.metadata,
        )
      : (response?.h3 ?? []).map((cell) => cell.power?.metadata);
  const provenance =
    [
      ...new Set(
        sources.flatMap((metadata) =>
          (metadata?.provenanceMix ?? []).map(
            (share) =>
              provenanceNames[
                share.provenance as keyof typeof provenanceNames
              ] ?? "Provenance unavailable",
          ),
        ),
      ),
    ].join(" · ") || "Provenance unavailable";
  const host = useRef<HTMLDivElement>(null);
  const renderer = useRef<ReturnType<typeof mountGrid> | null>(null);
  const select = useRef(onSelect);
  useEffect(() => {
    select.current = onSelect;
  }, [onSelect]);
  const [mode, setMode] = useState("Geographic fallback");
  useEffect(() => {
    if (!active) {
      setMode("Geographic field parked");
      return;
    }
    let cancelled = false;
    void import("./renderer").then(({ mountGrid }) => {
      if (cancelled || !host.current) return;
      try {
        renderer.current = mountGrid(host.current, (id) => select.current(id));
        setMode("3D geographic field");
      } catch {
        setMode("Geographic fallback · WebGL unavailable");
      }
    });
    return () => {
      cancelled = true;
      renderer.current?.dispose();
      renderer.current = null;
    };
  }, [active]);
  useEffect(() => {
    renderer.current?.update(projection.cells, selected);
  }, [projection, selected, mode]);
  return (
    <section
      className="living-grid"
      aria-label="Living Grid geography"
      hidden={!active}
    >
      <div className="field-header">
        <span className="eyebrow">The Living Grid</span>
        <span className="mono">{cells.length} H3 cells · LZ_AEN</span>
      </div>
      <FieldMeasure
        measure={measure}
        onChange={setMeasure}
        response={response}
      />
      <div className="grid-stage">
        <div className="field-caption">
          <h2>Greater Austin</h2>
          <p>{measureLabels[measure]}, in place.</p>
        </div>
        <div className="north-marker" aria-hidden="true">
          N <span>↗</span>
        </div>
        {projection.error ? (
          <p role="alert">{projection.error}</p>
        ) : (
          <Relief cells={projection.cells} />
        )}
        <div className="webgl-host" ref={host} />
        <div className="spatial-caption">
          <span className="mono">{mode}</span>
          <span>Height = absolute MW · Footprint = sites</span>
        </div>
      </div>
      <div className="field-footer">
        <span>
          {measure === "installed"
            ? "Neutral capacity · no availability inferred"
            : "Missing cells remain unknown · no fleet totals distributed"}
        </span>
        <span>{provenance}</span>
      </div>
      <GeographicTable
        cells={projection.error ? [] : cells}
        projected={projection.cells}
        selected={selected}
        onSelect={onSelect}
        measure={measure}
        response={response}
      />
    </section>
  );
}

function Relief({ cells }: { cells: GridCell[] }) {
  const span =
    Math.max(10, ...cells.flatMap((cell) => cell.position.map(Math.abs))) * 2.8;
  const transform = ([x, z]: [number, number], height = 0) => [
    x - z * 0.5,
    z * 0.55 + x * 0.22 - height,
  ];
  return (
    <svg
      className="grid-relief"
      viewBox={`${-span / 2} ${-span / 2} ${span} ${span}`}
      role="img"
      aria-label="H3 power relief, detailed values in the geographic data table"
    >
      <g>
        {[...cells]
          .sort((a, b) => a.position[1] - b.position[1])
          .map((cell) => {
            const boundary = cell.boundary.map(
              ([x, z]) =>
                [
                  cell.position[0] + (x - cell.position[0]) * cell.footprint,
                  cell.position[1] + (z - cell.position[1]) * cell.footprint,
                ] as [number, number],
            );
            return (
              <g
                key={cell.id}
                style={
                  {
                    "--cell-color": `#${cell.color.toString(16).padStart(6, "0")}`,
                  } as React.CSSProperties
                }
              >
                {boundary.map((point, i) => {
                  const next = boundary[(i + 1) % boundary.length];
                  return (
                    next && (
                      <polygon
                        className="relief-side"
                        key={i}
                        points={[
                          transform(point),
                          transform(next),
                          transform(next, cell.height),
                          transform(point, cell.height),
                        ]
                          .map((p) => p.join(","))
                          .join(" ")}
                      />
                    )
                  );
                })}
                <polygon
                  className="relief-top"
                  points={boundary
                    .map((point) => transform(point, cell.height).join(","))
                    .join(" ")}
                />
              </g>
            );
          })}
      </g>
    </svg>
  );
}

const measureSchema = z.enum([
  "installed",
  "sent",
  "acknowledged",
  "delivered",
]);
const measureLabels = {
  installed: "Installed capacity",
  sent: "Sent intent",
  acknowledged: "Acknowledged receipt",
  delivered: "Measured delivery",
};

function FieldMeasure({
  measure,
  onChange,
  response,
}: {
  measure: z.infer<typeof measureSchema>;
  onChange: (value: z.infer<typeof measureSchema>) => void;
  response?: EventSample;
}) {
  return (
    <div className="field-measure">
      <label>
        Geographic measure{" "}
        <select
          value={measure}
          onChange={(event) =>
            onChange(measureSchema.parse(event.target.value))
          }
        >
          {measureSchema.options.map((value) => (
            <option key={value} value={value}>
              {measureLabels[value]}
            </option>
          ))}
        </select>
      </label>
      {measure !== "installed" && (
        <span>
          {response
            ? `Observed ${new Date(response.time).toISOString()}`
            : "Event evidence unavailable"}
        </span>
      )}
    </div>
  );
}

function GeographicTable({
  cells,
  projected,
  selected,
  onSelect,
  measure,
  response,
}: {
  cells: H3SiteAggregate[];
  projected: GridCell[];
  selected: string | null;
  onSelect: (id: string) => void;
  measure: z.infer<typeof measureSchema>;
  response?: EventSample;
}) {
  const [sort, setSort] = useState("location");
  const rows = [...cells].sort((a, b) =>
    sort === "capacity"
      ? (b.installedMw?.value ?? 0) - (a.installedMw?.value ?? 0)
      : a.h3Cell.localeCompare(b.h3Cell),
  );

  return (
    <details className="geography-table">
      <summary>
        Inspect the geographic data{" "}
        <span className="mono">{cells.length} cells ↓</span>
      </summary>
      <label>
        Sort cells{" "}
        <select value={sort} onChange={(event) => setSort(event.target.value)}>
          <option value="location">H3 location</option>
          <option value="capacity">Installed power</option>
        </select>
      </label>
      <div
        className="table-scroll"
        role="region"
        aria-label="Geographic measurements"
        tabIndex={0}
      >
        <table>
          <thead>
            <tr>
              <th>H3 region</th>
              <th>Sites</th>
              <th>{measureLabels[measure]} MW</th>
              <th>Evidence</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((cell) => (
              <tr key={cell.h3Cell} aria-selected={selected === cell.h3Cell}>
                <td>
                  <button
                    onClick={() => onSelect(cell.h3Cell)}
                    className="text-button mono"
                  >
                    {cell.h3Cell}
                  </button>
                </td>
                <td>{cell.siteCount.toLocaleString()}</td>
                <td>
                  {projected
                    .find((item) => item.id === cell.h3Cell)
                    ?.value?.toFixed(3) ?? "Unavailable"}
                </td>
                <td>
                  <Evidence
                    metadata={
                      measure === "installed"
                        ? cell.installedMw?.metadata
                        : response?.h3.find(
                            (item) => item.h3Cell === cell.h3Cell,
                          )?.power?.metadata
                    }
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </details>
  );
}

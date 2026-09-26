import { useEffect, useMemo, useRef, useState } from "react";
import type { H3SiteAggregate } from "../api/gen/gridos/v1/api_pb";
import { projectCells, type GridCell } from "./scene";
import type { mountGrid } from "./renderer";
import { Evidence } from "../api/Provenance";

export default function LivingGrid({
  cells,
  selected,
  onSelect,
}: {
  cells: H3SiteAggregate[];
  selected: string | null;
  onSelect: (id: string) => void;
}) {
  const projection = useMemo(() => {
    try {
      return { cells: projectCells(cells), error: null };
    } catch {
      return {
        cells: [],
        error:
          "Geography cannot be displayed: invalid location or capacity evidence.",
      };
    }
  }, [cells]);
  const host = useRef<HTMLDivElement>(null);
  const renderer = useRef<ReturnType<typeof mountGrid> | null>(null);
  const [mode, setMode] = useState("Geographic fallback");
  const [sort, setSort] = useState("location");
  useEffect(() => {
    let cancelled = false;
    if (!projection.cells.length) return;
    void import("./renderer").then(({ mountGrid }) => {
      if (cancelled || !host.current) return;
      try {
        renderer.current = mountGrid(host.current, projection.cells, onSelect);
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
  }, [projection, onSelect]);
  useEffect(() => {
    renderer.current?.highlight(selected);
  }, [selected, mode]);
  const rows = (projection.error ? [] : [...cells]).sort((a, b) =>
    sort === "capacity"
      ? (b.installedMw?.value ?? 0) - (a.installedMw?.value ?? 0)
      : a.h3Cell.localeCompare(b.h3Cell),
  );
  return (
    <section className="living-grid" aria-label="Living Grid geography">
      <div className="field-header">
        <span className="eyebrow">The Living Grid</span>
        <span className="mono">{cells.length} H3 cells · LZ_AEN</span>
      </div>
      <div className="grid-stage">
        <div className="field-caption">
          <h2>Greater Austin</h2>
          <p>Installed capacity, in place.</p>
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
          <span>Height = installed MW · Footprint = sites</span>
        </div>
      </div>
      <div className="field-footer">
        <span>Neutral capacity · no availability inferred</span>
        <span>SIMULATED</span>
      </div>
      <details className="geography-table">
        <summary>
          Inspect the geographic data{" "}
          <span className="mono">{cells.length} cells ↓</span>
        </summary>
        <label>
          Sort cells{" "}
          <select
            value={sort}
            onChange={(event) => setSort(event.target.value)}
          >
            <option value="location">H3 location</option>
            <option value="capacity">Installed power</option>
          </select>
        </label>
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>H3 region</th>
                <th>Sites</th>
                <th>Installed MW</th>
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
                  <td>{cell.installedMw?.value.toFixed(3) ?? "—"}</td>
                  <td>
                    <Evidence metadata={cell.installedMw?.metadata} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </details>
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
      aria-label="H3 capacity relief, detailed values in the geographic data table"
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
              <g key={cell.id}>
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

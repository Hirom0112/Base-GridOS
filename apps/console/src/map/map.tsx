import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSession } from "../api/auth";
import { Evidence } from "../api/Provenance";
import {
  mapFeatures,
  mapMeasureSchema,
  mapMeasureLabels,
  type MapFeatures,
  type MapMeasure,
} from "./map-data";
import type { mountMap } from "./map-renderer";
import type { GeoCell } from "../api/gen/gridos/v1/geo_pb";
import "./map.css";

export function FleetMap() {
  const { client, identity } = useSession();
  const sites = useQuery({
    queryKey: ["geo-cells", identity.role, 7],
    queryFn: ({ signal }) =>
      client.geo.listCells(
        { resolution: 7, loadZones: ["LZ_AEN"] },
        { signal },
      ),
    refetchInterval: 5000,
  });
  const [measure, setMeasure] = useState<MapMeasure>("capacity");
  const [selected, setSelected] = useState<string | null>(null);
  const [status, setStatus] = useState("Preparing geographic map…");
  const host = useRef<HTMLDivElement>(null);
  const renderer = useRef<ReturnType<typeof mountMap> | null>(null);
  const projection = useMemo(() => {
    try {
      const cells = sites.data?.cells ?? [];
      return { cells, features: mapFeatures(cells), error: null };
    } catch {
      return {
        cells: [],
        features: mapFeatures([]),
        error: "Geographic evidence is invalid. No map values are displayed.",
      };
    }
  }, [sites.data]);
  useEffect(() => {
    let cancelled = false;
    void import("./map-renderer")
      .then(({ mountMap }) => {
        if (cancelled || !host.current) return;
        try {
          renderer.current = mountMap(host.current, setSelected, setStatus);
        } catch {
          setStatus("Map unavailable · geographic table retained");
        }
      })
      .catch(() => {
        if (!cancelled)
          setStatus("Map unavailable · geographic table retained");
      });
    return () => {
      cancelled = true;
      renderer.current?.dispose();
      renderer.current = null;
    };
  }, []);
  useEffect(() => {
    renderer.current?.update(projection.features, measure, selected);
  }, [projection, measure, selected, status]);
  const cell = projection.cells.find((item) => item.h3Cell === selected);
  return (
    <section className="fleet-map" aria-label="Geographic fleet exploration">
      <div className="map-toolbar">
        <label>
          Map measure{" "}
          <select
            value={measure}
            onChange={(event) =>
              setMeasure(mapMeasureSchema.parse(event.target.value))
            }
          >
            {mapMeasureSchema.options.map((value) => (
              <option key={value} value={value}>
                {mapMeasureLabels[value]}
              </option>
            ))}
          </select>
        </label>
        <span className="mode-chip">SIMULATED</span>
      </div>
      {(sites.isError || projection.error) && (
        <p role="alert">{projection.error ?? sites.error?.message}</p>
      )}
      {sites.isPending && <p role="status">Waiting for geographic evidence…</p>}
      <div className="map-stage" data-ready={status === "Geographic map ready"}>
        <MapOverview
          features={projection.features}
          measure={measure}
          selected={selected}
        />
        <div className="map-host" ref={host} />
      </div>
      <div className="map-caption">
        <p className="eyebrow">Greater Austin / LZ_AEN</p>
        <h2>A regional view.</h2>
        <p>Aggregate locations. Household privacy intact.</p>
      </div>
      <div className="map-legend">
        <span className="map-ramp" aria-hidden="true" />
        <span>
          Lighter = higher {mapMeasureLabels[measure].toLowerCase()}. Unknown
          charge is its own layer.
        </span>
      </div>
      <p role="status" className="map-status">
        {status} · {projection.cells.length} H3 regions
      </p>
      <p className="map-source">
        Texas outline: Census · CONFIRMED_PUBLIC. No street or terrain tiles.
        Simulated zone boundaries are hidden; they are not operational service
        areas.
      </p>
      {cell && (
        <section className="map-selection" aria-label="Selected map cell">
          <div>
            <p className="eyebrow">Selected H3 region</p>
            <h3 className="mono">{cell.h3Cell}</h3>
            <p>
              {cell.siteCount.toLocaleString()} sites · Aggregate location only
            </p>
          </div>
          <div>
            <strong>{mapMeasureLabels[measure]}</strong>
            <p className="mono">
              {projection.features.features
                .find((feature) => feature.id === selected)
                ?.properties[measure].toLocaleString()}
            </p>
            <Evidence metadata={cell.metadata} />
          </div>
          <button className="text-button" onClick={() => setSelected(null)}>
            Clear map selection
          </button>
        </section>
      )}
      <MapTable
        cells={projection.cells}
        features={projection.features}
        measure={measure}
        selected={selected}
        select={setSelected}
      />
    </section>
  );
}

function MapTable({
  cells,
  features,
  measure,
  selected,
  select,
}: {
  cells: GeoCell[];
  features: MapFeatures;
  measure: MapMeasure;
  selected: string | null;
  select: (id: string) => void;
}) {
  return (
    <details className="map-table" open>
      <summary>Geographic evidence · inspect without the map</summary>
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th>H3 region</th>
              <th>Sites</th>
              <th>{mapMeasureLabels[measure]}</th>
              <th>Evidence</th>
            </tr>
          </thead>
          <tbody>
            {[...features.features]
              .sort((a, b) => b.properties[measure] - a.properties[measure])
              .map((feature) => {
                const item = cells.find((cell) => cell.h3Cell === feature.id)!;
                return (
                  <tr
                    key={item.h3Cell}
                    aria-selected={selected === item.h3Cell}
                  >
                    <td>
                      <button
                        className="text-button mono"
                        aria-label={`Inspect ${item.h3Cell}`}
                        onClick={() => select(item.h3Cell)}
                      >
                        {item.h3Cell}
                      </button>
                    </td>
                    <td>{item.siteCount.toLocaleString()}</td>
                    <td>{feature.properties[measure].toLocaleString()}</td>
                    <td>
                      <Evidence metadata={item.metadata} />
                    </td>
                  </tr>
                );
              })}
          </tbody>
        </table>
      </div>
    </details>
  );
}

function MapOverview({
  features,
  measure,
  selected,
}: {
  features: MapFeatures;
  measure: MapMeasure;
  selected: string | null;
}) {
  const points = features.features.flatMap((feature) =>
    feature.geometry.coordinates.flat(),
  );
  if (!points.length) return null;
  const west = Math.min(...points.map((p) => p[0]));
  const north = Math.max(...points.map((p) => p[1]));
  const width = Math.max(...points.map((p) => p[0])) - west;
  const height = north - Math.min(...points.map((p) => p[1]));
  const maximum = Math.max(
    0.001,
    ...features.features.map((feature) => feature.properties[measure]),
  );
  return (
    <svg
      className="map-overview"
      viewBox={`-0.02 -0.02 ${width + 0.04} ${height + 0.04}`}
      role="img"
      aria-label="H3 geographic overview"
    >
      {features.features.map((feature) => (
        <polygon
          key={feature.id}
          fillOpacity={0.2 + (0.8 * feature.properties[measure]) / maximum}
          data-selected={selected === feature.id}
          points={feature.geometry.coordinates[0]!.map(
            ([lon, lat]) => `${lon - west},${north - lat}`,
          ).join(" ")}
        />
      ))}
    </svg>
  );
}

import {
  Map,
  NavigationControl,
  ScaleControl,
  setWorkerUrl,
  type GeoJSONSource,
} from "maplibre-gl";
import type { MapFeatures, MapMeasure } from "./map-data";
import "maplibre-gl/dist/maplibre-gl.css";
import workerUrl from "maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url";
import groundImagery from "../fleet/austin-ground.jpg";

setWorkerUrl(workerUrl);

export function mountMap(
  host: HTMLElement,
  select: (id: string) => void,
  status: (text: string) => void,
) {
  const map = new Map({
    container: host,
    style: "/geo/style.json",
    center: [-97.74, 30.27],
    zoom: 9,
    pitch: 0,
    bearing: 0,
    pixelRatio: Math.min(devicePixelRatio, 1.5),
    attributionControl: false,
    maxPitch: 60,
  });
  map.addControl(new NavigationControl({ visualizePitch: true }), "top-right");
  map.addControl(new ScaleControl({ unit: "metric" }), "bottom-right");
  let ready = false;
  let fitted = false;
  let disposed = false;
  let data: MapFeatures = { type: "FeatureCollection", features: [] };
  let measure: MapMeasure = "capacity";
  let selected: string | null = null;
  const timeout = window.setTimeout(fail, 15000);
  function dispose() {
    if (disposed) return;
    disposed = true;
    clearTimeout(timeout);
    resize.disconnect();
    map.remove();
  }
  function fail() {
    dispose();
    status("Map unavailable · geographic table retained");
  }
  async function paint() {
    if (!ready || disposed) return;
    try {
      await map.getSource<GeoJSONSource>("fleet")?.setData(data);
      if (disposed) return;
      const maximum = Math.max(
        0.001,
        ...data.features.map(
          (feature) => feature.properties[measure] / feature.properties.area,
        ),
      );
      map.setPaintProperty("fleet-fill", "fill-color", [
        "interpolate",
        ["linear"],
        ["/", ["get", measure], ["get", "area"]],
        0,
        "#1c2e25",
        maximum,
        "#8fe8b8",
      ]);
      map.setFilter("fleet-selection", ["==", ["get", "id"], selected ?? ""]);
      if (!fitted && data.features.length) {
        const points = data.features
          .filter((feature) => !feature.properties.coarse)
          .flatMap((feature) => feature.geometry.coordinates.flat());
        map.fitBounds(
          [
            [
              Math.min(...points.map((p) => p[0])),
              Math.min(...points.map((p) => p[1])),
            ],
            [
              Math.max(...points.map((p) => p[0])),
              Math.max(...points.map((p) => p[1])),
            ],
          ],
          {
            padding: 32,
            duration: 0,
          },
        );
        fitted = true;
      }
    } catch {
      if (!disposed) fail();
    }
  }
  const resize = new ResizeObserver(() => {
    if (!disposed) map.resize();
  });
  resize.observe(host);
  map.on("error", fail);
  map.on("load", () => {
    if (disposed) return;
    clearTimeout(timeout);
    addFleetLayers(map, data);
    map.on("click", "fleet-fill", (event) => {
      const id: unknown = event.features?.[0]?.properties.id;
      if (typeof id === "string") select(id);
    });
    ready = true;
    status("Geographic map ready");
    void paint();
  });
  return {
    update(
      next: MapFeatures,
      nextMeasure: MapMeasure,
      nextSelection: string | null,
    ) {
      data = next;
      measure = nextMeasure;
      selected = nextSelection;
      void paint();
    },
    dispose,
  };
}

function addFleetLayers(map: Map, data: MapFeatures) {
  map.setPaintProperty("background", "background-color", "#050907");
  map.setPaintProperty("texas-fill", "fill-color", "#09120e");
  map.setPaintProperty("texas-outline", "line-color", "#7f9588");
  map.addSource("ground", {
    type: "image",
    url: groundImagery,
    coordinates: [
      [-98.55, 31.0],
      [-96.95, 31.0],
      [-96.95, 29.6],
      [-98.55, 29.6],
    ],
  });
  map.addLayer({
    id: "ground",
    type: "raster",
    source: "ground",
    paint: { "raster-opacity": 0.9 },
  });
  map.setLayoutProperty("weather-zones", "visibility", "none");
  map.setLayoutProperty("load-zones", "visibility", "none");
  map.addSource("fleet", { type: "geojson", data });
  map.addLayer({
    id: "fleet-fill",
    type: "fill",
    source: "fleet",
    paint: {
      "fill-opacity": ["case", ["get", "coarse"], 0.08, 0.85],
    },
  });
  map.addLayer({
    id: "fleet-outline",
    type: "line",
    source: "fleet",
    filter: ["!", ["get", "coarse"]],
    paint: {
      "line-color": "#66f2a4",
      "line-width": 0.6,
      "line-opacity": 0.5,
    },
  });
  map.addLayer({
    id: "fleet-privacy",
    type: "line",
    source: "fleet",
    filter: ["get", "coarse"],
    paint: {
      "line-color": "#7f9588",
      "line-width": 1,
      "line-dasharray": [3, 2],
    },
  });
  map.addLayer({
    id: "fleet-selection",
    type: "line",
    source: "fleet",
    filter: ["==", ["get", "id"], ""],
    paint: { "line-color": "#f0f7f2", "line-width": 3 },
  });
}

import {
  Map,
  NavigationControl,
  ScaleControl,
  setWorkerUrl,
  type ExpressionSpecification,
  type GeoJSONSource,
} from "maplibre-gl";
import { fleetNetwork, type MapFeatures, type MapMeasure } from "./map-data";
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
      const network = fleetNetwork(data);
      await map.getSource<GeoJSONSource>("fleet-nodes")?.setData(network.nodes);
      await map.getSource<GeoJSONSource>("fleet-links")?.setData(network.links);
      if (disposed) return;
      const maximum = Math.max(
        0.001,
        ...data.features.map(
          (feature) => feature.properties[measure] / feature.properties.area,
        ),
      );
      map.setPaintProperty("fleet-node", "circle-color", [
        "interpolate",
        ["linear"],
        ["/", ["get", measure], ["get", "area"]],
        0,
        "#2b4a7a",
        maximum * 0.5,
        "#4ff0d2",
        maximum,
        "#e9fffb",
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
    map.on("mouseenter", "fleet-node", () => {
      map.getCanvas().style.cursor = "pointer";
    });
    map.on("mouseleave", "fleet-node", () => {
      map.getCanvas().style.cursor = "";
    });
    map.on("click", "fleet-node", (event) => {
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
  map.setPaintProperty("background", "background-color", "#04070d");
  map.setPaintProperty("texas-fill", "fill-color", "#070b14");
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
    paint: {
      "raster-opacity": 0.42,
      "raster-contrast": -0.25,
      "raster-resampling": "linear",
    },
  });
  map.setLayoutProperty("weather-zones", "visibility", "none");
  map.setLayoutProperty("load-zones", "visibility", "none");
  const network = fleetNetwork(data);
  map.addSource("fleet-links", { type: "geojson", data: network.links });
  map.addSource("fleet-nodes", { type: "geojson", data: network.nodes });
  map.addLayer({
    id: "fleet-privacy",
    type: "circle",
    source: "fleet-nodes",
    filter: ["get", "coarse"],
    paint: {
      "circle-radius": ["interpolate", ["linear"], ["zoom"], 8, 18, 12, 60],
      "circle-color": "#8492ad",
      "circle-opacity": 0.06,
      "circle-blur": 0.8,
      "circle-stroke-width": 1,
      "circle-stroke-color": "#8492ad",
      "circle-stroke-opacity": 0.25,
    },
  });
  map.addLayer({
    id: "fleet-link-glow",
    type: "line",
    source: "fleet-links",
    layout: { "line-cap": "round" },
    paint: {
      "line-color": "#4ff0d2",
      "line-width": 5,
      "line-opacity": 0.12,
      "line-blur": 4,
    },
  });
  map.addLayer({
    id: "fleet-links",
    type: "line",
    source: "fleet-links",
    layout: { "line-cap": "round" },
    paint: {
      "line-color": "#7ff3df",
      "line-width": 1,
      "line-opacity": 0.55,
    },
  });
  map.addLayer({
    id: "fleet-node-glow",
    type: "circle",
    source: "fleet-nodes",
    filter: ["!", ["get", "coarse"]],
    paint: {
      "circle-radius": nodeRadius(3, 0),
      "circle-color": "#4ff0d2",
      "circle-opacity": 0.22,
      "circle-blur": 1,
    },
  });
  map.addLayer({
    id: "fleet-node",
    type: "circle",
    source: "fleet-nodes",
    filter: ["!", ["get", "coarse"]],
    paint: {
      "circle-radius": nodeRadius(1, 0),
      "circle-stroke-width": 1,
      "circle-stroke-color": "#04070d",
    },
  });
  map.addLayer({
    id: "fleet-selection",
    type: "circle",
    source: "fleet-nodes",
    filter: ["==", ["get", "id"], ""],
    paint: {
      "circle-radius": nodeRadius(1, 6),
      "circle-color": "transparent",
      "circle-stroke-width": 2,
      "circle-stroke-color": "#eef3fb",
    },
  });
}

function nodeRadius(scale: number, extra: number): ExpressionSpecification {
  return [
    "interpolate",
    ["linear"],
    ["zoom"],
    8,
    [
      "+",
      extra,
      ["*", scale, ["+", 1.5, ["*", 0.45, ["sqrt", ["get", "sites"]]]]],
    ],
    12,
    [
      "+",
      extra,
      ["*", scale, ["+", 3, ["*", 1.1, ["sqrt", ["get", "sites"]]]]],
    ],
  ];
}

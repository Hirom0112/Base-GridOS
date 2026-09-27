import * as THREE from "three";
import type { GridCell } from "./scene";

export type View = { x: number; z: number; span: number; pitch: number };

export function shouldFly(from: View, to: View) {
  return (
    Math.abs(from.span - to.span) / Math.max(from.span, to.span) > 0.2 &&
    !matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

export function fly(
  from: View,
  to: View,
  frame: (view: View, at: number) => void,
  done: () => void,
) {
  let handle = 0;
  const started = performance.now();
  const step = (now: number) => {
    const t = Math.min(1, (now - started) / 900);
    const eased = t < 0.5 ? 4 * t ** 3 : 1 - (-2 * t + 2) ** 3 / 2;
    frame(
      {
        x: from.x + (to.x - from.x) * eased,
        z: from.z + (to.z - from.z) * eased,
        span: from.span + (to.span - from.span) * eased,
        pitch: from.pitch + (to.pitch - from.pitch) * eased,
      },
      now,
    );
    if (t < 1) handle = requestAnimationFrame(step);
    else done();
  };
  handle = requestAnimationFrame(step);
  return () => cancelAnimationFrame(handle);
}

export function frameCamera(
  camera: THREE.PerspectiveCamera,
  view: View,
  aspect: number,
) {
  const fit = view.span / Math.tan(THREE.MathUtils.degToRad(camera.fov / 2));
  const distance = fit * (aspect < 1.25 ? 1.3 / aspect : 1.02);
  const pitch = THREE.MathUtils.degToRad(
    aspect < 1 ? view.pitch + 8 : view.pitch,
  );
  const heading = THREE.MathUtils.degToRad(-14);
  const lookX = view.x + (aspect < 1 ? 0 : -view.span * 0.26);
  const lookZ = view.z + view.span * 0.2;
  camera.aspect = aspect;
  camera.far = Math.max(distance * 4, 400);
  camera.position.set(
    lookX + Math.sin(heading) * Math.cos(pitch) * distance,
    Math.sin(pitch) * distance,
    lookZ + Math.cos(heading) * Math.cos(pitch) * distance,
  );
  camera.lookAt(lookX, 0, lookZ);
  camera.updateProjectionMatrix();
}

export function placeOverlay(
  overlay: HTMLElement,
  camera: THREE.PerspectiveCamera,
  places: { name: string; position: [number, number] }[],
  width: number,
  height: number,
) {
  const screen = (x: number, z: number) => {
    const point = new THREE.Vector3(x, 0, z).project(camera);
    return {
      x: ((point.x + 1) / 2) * width,
      y: ((1 - point.y) / 2) * height,
    };
  };
  const placements = places
    .map(({ name, position }) => ({ name, ...screen(...position) }))
    .filter(
      ({ x, y }) => x > 40 && x < width - 40 && y > 40 && y < height - 40,
    );
  const origin = screen(0, 0);
  const north = screen(0, -10);
  const east = screen(10, 0);
  const pixelsPerKm = Math.hypot(east.x - origin.x, east.y - origin.y) / 10;
  const kilometers =
    [5, 10, 20, 40, 80].find((value) => value * pixelsPerKm >= 72) ?? 80;
  const bearing = Math.atan2(north.x - origin.x, origin.y - north.y);
  overlay.replaceChildren(
    ...placements.map(({ name, x, y }) => {
      const label = document.createElement("span");
      label.className = "place-label";
      label.textContent = name;
      label.style.transform = `translate(${x}px, ${y}px)`;
      return label;
    }),
    compass(bearing),
    scaleBar(kilometers, kilometers * pixelsPerKm),
  );
}

function compass(bearing: number) {
  const element = document.createElement("span");
  element.className = "field-compass";
  element.textContent = "N";
  element.style.setProperty("--bearing", `${bearing}rad`);
  return element;
}

function scaleBar(kilometers: number, pixels: number) {
  const element = document.createElement("span");
  element.className = "field-scale";
  element.textContent = `${kilometers} km`;
  element.style.width = `${Math.round(pixels)}px`;
  return element;
}

export function viewFor(cells: GridCell[], selected: string | null): View {
  const focus = cells.find((cell) => cell.id === selected);
  const corner = focus?.boundary[0];
  if (focus && corner)
    return {
      x: focus.position[0],
      z: focus.position[1],
      span:
        Math.hypot(
          corner[0] - focus.position[0],
          corner[1] - focus.position[1],
        ) * 5.5,
      pitch: 42,
    };
  return {
    x: 0,
    z: 0,
    span: Math.max(
      12,
      ...cells.map((cell) => Math.hypot(...cell.position) + 3),
    ),
    pitch: 50,
  };
}

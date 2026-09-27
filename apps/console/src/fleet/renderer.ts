import * as THREE from "three";
import type { GridCell } from "./scene";
import { buildGround, lineGeometry } from "./ground";
import { buildHomes } from "./homes";
import {
  fly,
  frameCamera,
  placeOverlay,
  shouldFly,
  viewFor,
  type View,
} from "./camera";
import { createRenderQuality } from "./render-quality";
import { animateResponse } from "./response-motion";

const hexagon = new THREE.CylinderGeometry(1, 1, 1, 6);
const plate = new THREE.CylinderGeometry(1, 1, 0.04, 6);

export function mountGrid(host: HTMLElement, select: (id: string) => void) {
  const renderer = new THREE.WebGLRenderer({ alpha: true, antialias: true });
  const quality = createRenderQuality(
    window.devicePixelRatio,
    window.innerWidth,
  );
  renderer.setPixelRatio(quality.pixelRatio);
  renderer.domElement.setAttribute("aria-hidden", "true");
  renderer.domElement.dataset.livingGrid = "true";
  host.append(renderer.domElement);
  const overlay = document.createElement("div");
  overlay.className = "field-overlay";
  overlay.setAttribute("aria-hidden", "true");
  host.append(overlay);
  const scene = new THREE.Scene();
  scene.fog = new THREE.FogExp2(0x04070d, 0.0032);
  const camera = new THREE.PerspectiveCamera(30, 1, 0.5, 4000);
  scene.add(new THREE.HemisphereLight(0xdfe8ff, 0x04070d, 2.2));
  const light = new THREE.DirectionalLight(0xffffff, 2.4);
  light.position.set(-40, 80, 30);
  scene.add(light);
  const bodies = beamMaterial();
  let cells: GridCell[] = [];
  let field = buildField([], null, bodies);
  let ground = buildGround([], () => render());
  let view: View = { x: 0, z: 0, span: 20, pitch: 50 };
  let cancelFlight = () => {};
  let cancelMotion = () => {};
  scene.add(field.group, ground.group);
  let visible = true;
  function render(at?: number) {
    if (!visible || document.hidden) {
      quality.frame();
      return;
    }
    const ratio = quality.frame(at);
    if (ratio !== renderer.getPixelRatio()) renderer.setPixelRatio(ratio);
    renderer.render(scene, camera);
  }
  const documentVisibility = () => render();
  function resizeFrame() {
    const width = host.clientWidth;
    const height = host.clientHeight;
    if (!width || !height) return;
    frameCamera(camera, view, width / height);
    renderer.setSize(width, height);
    placeOverlay(overlay, camera, ground.places, width, height);
    render();
  }
  const resize = new ResizeObserver(resizeFrame);
  resize.observe(host);
  const visibility = new IntersectionObserver(([entry]) => {
    visible = entry?.isIntersecting ?? false;
    render();
  });
  visibility.observe(host);
  document.addEventListener("visibilitychange", documentVisibility);
  const pointer = bindPointer(
    host,
    renderer.domElement,
    camera,
    () => field,
    select,
  );
  renderer.domElement.addEventListener("pointerup", pointer.pick);
  return {
    update(next: GridCell[], selected: string | null) {
      cancelMotion();
      const previous = new Map(cells.map((cell) => [cell.id, cell]));
      const groundChanged =
        next.length !== cells.length ||
        next.some((cell, index) => cell.id !== cells[index]?.id);
      cells = next;
      scene.remove(field.group);
      field.dispose();
      field = buildField(cells, selected, bodies);
      scene.add(field.group);
      if (groundChanged) {
        scene.remove(ground.group);
        ground.dispose();
        ground = buildGround(cells, () => render());
        scene.add(ground.group);
      }
      const target = viewFor(field.cells, selected);
      cancelFlight();
      if (!shouldFly(view, target)) {
        view = target;
        resizeFrame();
      } else {
        overlay.classList.add("in-flight");
        cancelFlight = fly(
          view,
          target,
          (next, at) => {
            view = next;
            frameCamera(camera, view, host.clientWidth / host.clientHeight);
            render(at);
          },
          () => {
            overlay.classList.remove("in-flight");
            resizeFrame();
          },
        );
      }
      const changed = field.cells.flatMap((cell, index) =>
        cell.response &&
        (cell.value !== previous.get(cell.id)?.value ||
          cell.response !== previous.get(cell.id)?.response)
          ? [index]
          : [],
      );
      cancelMotion = animateResponse(
        field.columns,
        changed,
        render,
        () => visible,
      );
    },
    dispose() {
      cancelMotion();
      cancelFlight();
      resize.disconnect();
      visibility.disconnect();
      document.removeEventListener("visibilitychange", documentVisibility);
      renderer.domElement.removeEventListener("pointerup", pointer.pick);
      pointer.dispose();
      field.dispose();
      ground.dispose();
      bodies.dispose();
      renderer.dispose();
      renderer.forceContextLoss();
      renderer.domElement.remove();
      overlay.remove();
    },
  };
}

function buildField(
  cells: GridCell[],
  selected: string | null,
  bodies: THREE.MeshBasicMaterial,
) {
  const group = new THREE.Group();
  const fine = cells.filter((cell) => !cell.coarse);
  const plates = new THREE.InstancedMesh(
    plate,
    new THREE.MeshBasicMaterial({ color: 0xffffff }),
    fine.length,
  );
  const tallest = Math.max(0, ...fine.map((cell) => cell.height));
  const strength = (cell: GridCell) =>
    tallest > 0 ? cell.height / tallest : 0;
  const columns = new THREE.InstancedMesh(hexagon, bodies, fine.length);
  const matrix = new THREE.Object3D();
  const edges: number[] = [];
  const edgeColors: number[] = [];
  for (const [index, cell] of fine.entries()) {
    const [x, z] = cell.position;
    const radius = cellRadius(cell);
    matrix.position.set(x, 0, z);
    matrix.rotation.y = cellRotation(cell);
    matrix.scale.set(radius * 0.98, 1, radius * 0.98);
    matrix.updateMatrix();
    plates.setMatrixAt(index, matrix.matrix);
    const height = Math.max(cell.height, 0.001);
    matrix.position.set(x, height / 2, z);
    matrix.scale.set(radius * 0.12, height, radius * 0.12);
    matrix.updateMatrix();
    columns.setMatrixAt(index, matrix.matrix);
    const tone = cellTone(cell, selected);
    columns.setColorAt(index, tone.edge);
    plates.setColorAt(
      index,
      new THREE.Color(0x0b1424).lerp(new THREE.Color(0x1f7a80), strength(cell)),
    );
    outlinePad(
      cell,
      tone.edge.clone().multiplyScalar(0.3 + 0.7 * strength(cell)),
      edges,
      edgeColors,
    );
  }
  const lines = new THREE.LineSegments(
    lineGeometry(edges, edgeColors),
    new THREE.LineBasicMaterial({
      vertexColors: true,
      transparent: true,
      opacity: 0.95,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
    }),
  );
  const privacy = coarseOutlines(cells.filter((cell) => cell.coarse));
  const homes = buildHomes(fine, strength);
  group.add(plates, homes.group, columns, lines, privacy);
  return {
    group,
    cells: fine,
    columns,
    plates,
    dispose() {
      plates.material.dispose();
      plates.dispose();
      columns.dispose();
      lines.geometry.dispose();
      lines.material.dispose();
      privacy.geometry.dispose();
      privacy.material.dispose();
      homes.dispose();
    },
  };
}

const responseColors = {
  sent: 0xf0f7f2,
  acknowledged: 0x6db8ff,
  delivered: 0x4ff0d2,
};

function cellTone(cell: GridCell, selected: string | null) {
  const ink = new THREE.Color(0x0c1322);
  if (cell.id === selected)
    return {
      body: new THREE.Color(0x9aa6c0),
      edge: new THREE.Color(0xffffff),
    };
  if (cell.response) {
    const state = new THREE.Color(responseColors[cell.response]);
    return { body: state.clone().lerp(ink, 0.55), edge: state };
  }
  if (cell.value === null)
    return {
      body: new THREE.Color(0x222b3b),
      edge: new THREE.Color(0x28324a),
    };
  return {
    body: new THREE.Color(0x3a4a66),
    edge: new THREE.Color(0x6fe9d6),
  };
}

function cellRadius(cell: GridCell) {
  const corner = cell.boundary[0];
  return corner
    ? Math.hypot(corner[0] - cell.position[0], corner[1] - cell.position[1])
    : 0;
}

function cellRotation(cell: GridCell) {
  const corner = cell.boundary[0];
  return corner
    ? Math.atan2(corner[0] - cell.position[0], corner[1] - cell.position[1])
    : 0;
}

function outlinePad(
  cell: GridCell,
  color: THREE.Color,
  edges: number[],
  colors: number[],
) {
  const [x, z] = cell.position;
  const corners = cell.boundary.map(([cx, cz]) => [
    x + (cx - x) * 0.96,
    z + (cz - z) * 0.96,
  ]);
  for (const [index, [ax = 0, az = 0]] of corners.entries()) {
    const [bx = 0, bz = 0] = corners[(index + 1) % corners.length] ?? [];
    edges.push(ax, 0.05, az, bx, 0.05, bz);
    colors.push(color.r, color.g, color.b, color.r, color.g, color.b);
  }
}

function coarseOutlines(coarse: GridCell[]) {
  const vertices = coarse.flatMap((cell) =>
    cell.boundary.flatMap(([x, z], index) => {
      const [nx, nz] = cell.boundary[(index + 1) % cell.boundary.length] ?? [];
      return nx === undefined || nz === undefined
        ? []
        : [x, 0.02, z, nx, 0.02, nz];
    }),
  );
  const lines = new THREE.LineSegments(
    lineGeometry(vertices),
    new THREE.LineDashedMaterial({
      color: 0x5c6678,
      dashSize: 0.8,
      gapSize: 0.6,
      transparent: true,
      opacity: 0.7,
    }),
  );
  lines.computeLineDistances();
  return lines;
}

function bindPointer(
  host: HTMLElement,
  canvas: HTMLCanvasElement,
  camera: THREE.Camera,
  field: () => ReturnType<typeof buildField>,
  select: (id: string) => void,
) {
  const chip = document.createElement("div");
  chip.className = "field-chip";
  chip.hidden = true;
  chip.setAttribute("aria-hidden", "true");
  host.append(chip);
  const ray = new THREE.Raycaster();
  function cellAt(event: PointerEvent) {
    const rect = host.getBoundingClientRect();
    ray.setFromCamera(
      new THREE.Vector2(
        ((event.clientX - rect.left) / rect.width) * 2 - 1,
        (-(event.clientY - rect.top) / rect.height) * 2 + 1,
      ),
      camera,
    );
    const hit = ray.intersectObjects([field().columns, field().plates])[0];
    return hit?.instanceId === undefined
      ? undefined
      : field().cells[hit.instanceId];
  }
  let pressed: { x: number; y: number } | null = null;
  function press(event: PointerEvent) {
    pressed = { x: event.clientX, y: event.clientY };
  }
  function pick(event: PointerEvent) {
    const moved = pressed
      ? Math.hypot(event.clientX - pressed.x, event.clientY - pressed.y)
      : 0;
    pressed = null;
    const cell = moved < 8 ? cellAt(event) : undefined;
    if (cell) select(cell.id);
  }
  function hover(event: PointerEvent) {
    const cell = event.buttons ? undefined : cellAt(event);
    canvas.style.cursor = cell ? "pointer" : "default";
    chip.hidden = !cell;
    if (!cell) return;
    const rect = host.getBoundingClientRect();
    chip.textContent = `${cell.sites.toLocaleString()} batteries · ${
      cell.value === null ? "unknown" : `${cell.value.toFixed(2)} MW`
    }`;
    chip.style.transform = `translate(${event.clientX - rect.left}px, ${
      event.clientY - rect.top - 16
    }px)`;
  }
  function leave() {
    chip.hidden = true;
  }
  canvas.addEventListener("pointerdown", press);
  canvas.addEventListener("pointermove", hover);
  canvas.addEventListener("pointerleave", leave);
  return {
    pick,
    dispose() {
      canvas.removeEventListener("pointerdown", press);
      canvas.removeEventListener("pointermove", hover);
      canvas.removeEventListener("pointerleave", leave);
      chip.remove();
    },
  };
}

function beamMaterial() {
  const material = new THREE.MeshBasicMaterial({
    color: 0xffffff,
    transparent: true,
    blending: THREE.AdditiveBlending,
    depthWrite: false,
  });
  material.onBeforeCompile = (shader) => {
    shader.vertexShader = shader.vertexShader
      .replace("void main() {", "varying float vRise;\nvoid main() {")
      .replace(
        "#include <begin_vertex>",
        "#include <begin_vertex>\nvRise = position.y + 0.5;",
      );
    shader.fragmentShader = shader.fragmentShader
      .replace("void main() {", "varying float vRise;\nvoid main() {")
      .replace(
        "#include <color_fragment>",
        "#include <color_fragment>\ndiffuseColor.a *= mix(0.8, 0.0, vRise);",
      );
  };
  return material;
}

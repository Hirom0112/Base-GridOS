import * as THREE from "three";
import type { GridCell } from "./scene";
import { buildGround, lineGeometry } from "./ground";
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
  const camera = new THREE.PerspectiveCamera(30, 1, 0.5, 4000);
  scene.add(new THREE.HemisphereLight(0xdff5e8, 0x040907, 2.2));
  const light = new THREE.DirectionalLight(0xffffff, 2.4);
  light.position.set(-40, 80, 30);
  scene.add(light);
  const bodies = new THREE.MeshStandardMaterial({
    color: 0xffffff,
    metalness: 0.2,
    roughness: 0.5,
  });
  bodies.onBeforeCompile = (shader) => {
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
        "#include <color_fragment>\ndiffuseColor.rgb *= mix(0.28, 1.15, vRise);",
      );
  };
  let cells: GridCell[] = [];
  let field = buildField([], null, bodies);
  let ground = buildGround([], () => render());
  let span = 20;
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
    frameCamera(camera, span, width / height);
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
  const ray = new THREE.Raycaster();
  function pick(event: PointerEvent) {
    const rect = host.getBoundingClientRect();
    ray.setFromCamera(
      new THREE.Vector2(
        ((event.clientX - rect.left) / rect.width) * 2 - 1,
        (-(event.clientY - rect.top) / rect.height) * 2 + 1,
      ),
      camera,
    );
    const hit = ray.intersectObjects([field.columns, field.plates])[0];
    const cell =
      hit?.instanceId === undefined ? undefined : field.cells[hit.instanceId];
    if (cell) select(cell.id);
  }
  renderer.domElement.addEventListener("pointerup", pick);
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
      span = Math.max(
        12,
        ...cells
          .filter((cell) => !cell.coarse)
          .map((cell) => Math.hypot(...cell.position) + 3),
      );
      resizeFrame();
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
      resize.disconnect();
      visibility.disconnect();
      document.removeEventListener("visibilitychange", documentVisibility);
      renderer.domElement.removeEventListener("pointerup", pick);
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

function frameCamera(
  camera: THREE.PerspectiveCamera,
  span: number,
  aspect: number,
) {
  const fit = span / Math.tan(THREE.MathUtils.degToRad(camera.fov / 2));
  const distance = fit * (aspect < 1.25 ? 1.3 / aspect : 1.02);
  const pitch = THREE.MathUtils.degToRad(aspect < 1 ? 58 : 50);
  const heading = THREE.MathUtils.degToRad(-14);
  camera.aspect = aspect;
  camera.far = distance * 4;
  camera.position.set(
    Math.sin(heading) * Math.cos(pitch) * distance,
    Math.sin(pitch) * distance,
    Math.cos(heading) * Math.cos(pitch) * distance,
  );
  camera.lookAt(aspect < 1 ? 0 : -span * 0.26, 0, span * 0.2);
  camera.updateProjectionMatrix();
}

function placeOverlay(
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

function buildField(
  cells: GridCell[],
  selected: string | null,
  bodies: THREE.MeshStandardMaterial,
) {
  const group = new THREE.Group();
  const fine = cells.filter((cell) => !cell.coarse);
  const plates = new THREE.InstancedMesh(
    plate,
    new THREE.MeshBasicMaterial({ color: 0x14231c }),
    fine.length,
  );
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
    matrix.scale.set(radius * cell.footprint, height, radius * cell.footprint);
    matrix.updateMatrix();
    columns.setMatrixAt(index, matrix.matrix);
    const tone = cellTone(cell, selected);
    columns.setColorAt(index, tone.body);
    if (cell.height > 0) outlineColumn(cell, tone.edge, edges, edgeColors);
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
  group.add(plates, columns, lines, privacy);
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
    },
  };
}

const responseColors = {
  sent: 0xf0f7f2,
  acknowledged: 0x6db8ff,
  delivered: 0x66f2a4,
};

function cellTone(cell: GridCell, selected: string | null) {
  const ink = new THREE.Color(0x0e1b15);
  if (cell.id === selected)
    return {
      body: new THREE.Color(0x8a9a92),
      edge: new THREE.Color(0xffffff),
    };
  if (cell.response) {
    const state = new THREE.Color(responseColors[cell.response]);
    return { body: state.clone().lerp(ink, 0.55), edge: state };
  }
  if (cell.value === null)
    return {
      body: new THREE.Color(0x303a35),
      edge: new THREE.Color(0x2c3631),
    };
  return {
    body: new THREE.Color(0x3d4a44),
    edge: new THREE.Color(0x8fc4a8),
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

function outlineColumn(
  cell: GridCell,
  color: THREE.Color,
  edges: number[],
  colors: number[],
) {
  const [x, z] = cell.position;
  const corners = cell.boundary.map(([cx, cz]) => [
    x + (cx - x) * cell.footprint,
    z + (cz - z) * cell.footprint,
  ]);
  const dim = color.clone().multiplyScalar(0.28);
  for (const [index, [ax = 0, az = 0]] of corners.entries()) {
    const [bx = 0, bz = 0] = corners[(index + 1) % corners.length] ?? [];
    edges.push(ax, cell.height, az, bx, cell.height, bz);
    colors.push(color.r, color.g, color.b, color.r, color.g, color.b);
    edges.push(ax, 0, az, ax, cell.height, az);
    colors.push(
      dim.r,
      dim.g,
      dim.b,
      color.r * 0.5,
      color.g * 0.5,
      color.b * 0.5,
    );
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
      color: 0x66736c,
      dashSize: 0.8,
      gapSize: 0.6,
      transparent: true,
      opacity: 0.7,
    }),
  );
  lines.computeLineDistances();
  return lines;
}

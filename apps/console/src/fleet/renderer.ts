import * as THREE from "three";
import type { GridCell } from "./scene";

export function mountGrid(host: HTMLElement, select: (id: string) => void) {
  const renderer = new THREE.WebGLRenderer({ alpha: true, antialias: true });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.5));
  renderer.domElement.setAttribute("aria-hidden", "true");
  renderer.domElement.dataset.livingGrid = "true";
  host.append(renderer.domElement);
  const scene = new THREE.Scene();
  let cells: GridCell[] = [];
  let span = 12;
  const camera = new THREE.OrthographicCamera(
    -span,
    span,
    span,
    -span,
    0.1,
    span * 10,
  );
  camera.position.set(span * 0.12, span * 0.95, span * 1.2);
  camera.lookAt(0, 0, 0);
  scene.add(new THREE.HemisphereLight(0xffffff, 0x253b31, 3));
  const light = new THREE.DirectionalLight(0xffffff, 3);
  light.position.set(-20, 40, 10);
  scene.add(light);
  const geometry = new THREE.CylinderGeometry(1, 1, 1, 6);
  const material = new THREE.MeshStandardMaterial({
    color: 0xa2afa8,
    metalness: 0.3,
    roughness: 0.55,
  });
  let mesh = new THREE.InstancedMesh(geometry, material, 0);
  scene.add(mesh);
  const grid = new THREE.GridHelper(1, 24, 0x405c4d, 0x263c30);
  grid.position.y = -0.04;
  grid.material.transparent = true;
  grid.material.opacity = 0.3;
  scene.add(grid);
  const boundaries = new THREE.LineSegments(
    new THREE.BufferGeometry(),
    new THREE.LineBasicMaterial({
      color: 0x66736c,
      transparent: true,
      opacity: 0.35,
    }),
  );
  scene.add(boundaries);
  let visible = true;
  function render() {
    if (visible && !document.hidden) renderer.render(scene, camera);
  }
  function resizeFrame() {
    const width = host.clientWidth;
    const height = host.clientHeight;
    if (!width || !height) return;
    const aspect = width / height;
    camera.left = -span * 0.56 * aspect;
    camera.right = span * 0.56 * aspect;
    camera.top = span * 0.56;
    camera.bottom = -span * 0.56;
    camera.far = span * 10;
    camera.position.set(span * 0.12, span * 0.95, span * 1.2);
    camera.lookAt(0, 0, 0);
    camera.updateProjectionMatrix();
    renderer.setSize(width, height);
    render();
  }
  const resize = new ResizeObserver(resizeFrame);
  resize.observe(host);
  const visibility = new IntersectionObserver(([entry]) => {
    visible = entry?.isIntersecting ?? false;
    render();
  });
  visibility.observe(host);
  document.addEventListener("visibilitychange", render);
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
    const hit = ray.intersectObject(mesh)[0];
    const cell =
      hit?.instanceId === undefined ? undefined : cells[hit.instanceId];
    if (cell) select(cell.id);
  }
  renderer.domElement.addEventListener("pointerup", pick);
  return {
    update(next: GridCell[], selected: string | null) {
      cells = next;
      scene.remove(mesh);
      mesh.dispose();
      mesh = capacityMesh(cells, geometry, material, selected);
      scene.add(mesh);
      boundaries.geometry.dispose();
      boundaries.geometry = cellBoundaries(cells);
      span =
        Math.max(
          10,
          ...cells.flatMap((cell) =>
            cell.position.map((value) => Math.abs(value) * 2),
          ),
        ) * 1.08;
      grid.scale.setScalar(span * 1.2);
      resizeFrame();
    },
    dispose() {
      resize.disconnect();
      visibility.disconnect();
      document.removeEventListener("visibilitychange", render);
      renderer.domElement.removeEventListener("pointerup", pick);
      geometry.dispose();
      material.dispose();
      mesh.dispose();
      boundaries.geometry.dispose();
      boundaries.material.dispose();
      grid.geometry.dispose();
      grid.material.dispose();
      renderer.dispose();
      renderer.forceContextLoss();
      renderer.domElement.remove();
    },
  };
}

function cellBoundaries(cells: GridCell[]) {
  const vertices = cells.flatMap((cell) =>
    cell.boundary.flatMap(([x, z], index) => {
      const next = cell.boundary[(index + 1) % cell.boundary.length];
      return next ? [x, 0, z, next[0], 0, next[1]] : [];
    }),
  );
  const geometry = new THREE.BufferGeometry();
  geometry.setAttribute(
    "position",
    new THREE.Float32BufferAttribute(vertices, 3),
  );
  return geometry;
}

function capacityMesh(
  cells: GridCell[],
  geometry: THREE.CylinderGeometry,
  material: THREE.MeshStandardMaterial,
  selected: string | null,
) {
  const mesh = new THREE.InstancedMesh(geometry, material, cells.length);
  const matrix = new THREE.Object3D();
  for (const [index, cell] of cells.entries()) {
    const corner = cell.boundary[0];
    if (!corner) continue;
    const dx = corner[0] - cell.position[0];
    const dz = corner[1] - cell.position[1];
    const radius = Math.hypot(dx, dz) * cell.footprint;
    matrix.position.set(cell.position[0], cell.height / 2, cell.position[1]);
    matrix.rotation.y = Math.atan2(dx, dz);
    matrix.scale.set(radius, cell.height, radius);
    matrix.updateMatrix();
    mesh.setMatrixAt(index, matrix.matrix);
    mesh.setColorAt(
      index,
      new THREE.Color(cell.id === selected ? 0xffffff : 0xa6b9ae),
    );
  }
  return mesh;
}

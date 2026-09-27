import * as THREE from "three";
import { mergeGeometries } from "three/examples/jsm/utils/BufferGeometryUtils.js";
import type { GridCell } from "./scene";

const walls = new THREE.Color(0xffc27a);
const roofs = new THREE.Color(0x141c2c);

function tinted(geometry: THREE.BufferGeometry, color: THREE.Color) {
  const flat = geometry.index ? geometry.toNonIndexed() : geometry;
  const count = flat.getAttribute("position").count;
  flat.setAttribute(
    "color",
    new THREE.Float32BufferAttribute(
      Array.from({ length: count }, () => [color.r, color.g, color.b]).flat(),
      3,
    ),
  );
  return flat;
}

function houseGeometry() {
  const body = new THREE.BoxGeometry(1, 0.62, 0.78);
  body.translate(0, 0.31, 0);
  const gable = new THREE.Shape();
  gable.moveTo(-0.6, 0);
  gable.lineTo(0.6, 0);
  gable.lineTo(0, 0.46);
  gable.closePath();
  const roof = new THREE.ExtrudeGeometry(gable, {
    depth: 0.9,
    bevelEnabled: false,
  });
  roof.translate(0, 0.62, -0.45);
  const merged = mergeGeometries([
    tinted(body, walls.clone().multiplyScalar(0.55)),
    tinted(roof, roofs),
  ]);
  body.dispose();
  roof.dispose();
  return merged;
}

const house = houseGeometry();
const battery = new THREE.BoxGeometry(0.22, 0.46, 0.22);
battery.translate(0, 0.23, 0);

function homesFor(sites: number) {
  if (sites < 15) return 1;
  if (sites < 45) return 2;
  return 3;
}

export function buildHomes(
  cells: GridCell[],
  strength: (cell: GridCell) => number,
) {
  const placements = cells.flatMap((cell) => {
    const corner = cell.boundary[0];
    if (!corner) return [];
    const radius = Math.hypot(
      corner[0] - cell.position[0],
      corner[1] - cell.position[1],
    );
    const count = homesFor(cell.sites);
    const turn = Math.atan2(
      corner[0] - cell.position[0],
      corner[1] - cell.position[1],
    );
    return Array.from({ length: count }, (_, index) => {
      const angle = turn + (index * Math.PI * 2) / 3 + Math.PI / 2;
      const offset = count === 1 ? 0 : radius * 0.42;
      return {
        cell,
        x: cell.position[0] + Math.sin(angle) * offset,
        z: cell.position[1] + Math.cos(angle) * offset,
        size: radius * 0.4,
        heading: angle,
      };
    });
  });
  const homes = new THREE.InstancedMesh(
    house,
    new THREE.MeshBasicMaterial({ vertexColors: true }),
    placements.length,
  );
  const batteries = new THREE.InstancedMesh(
    battery,
    new THREE.MeshBasicMaterial({ color: 0xffffff }),
    placements.length,
  );
  const matrix = new THREE.Object3D();
  const teal = new THREE.Color(0x4ff0d2);
  const idle = new THREE.Color(0x1d3450);
  for (const [index, place] of placements.entries()) {
    matrix.position.set(place.x, 0.05, place.z);
    matrix.rotation.set(0, place.heading, 0);
    matrix.scale.setScalar(place.size);
    matrix.updateMatrix();
    homes.setMatrixAt(index, matrix.matrix);
    matrix.position.set(
      place.x + Math.cos(place.heading) * place.size * 0.72,
      0.05,
      place.z - Math.sin(place.heading) * place.size * 0.72,
    );
    matrix.updateMatrix();
    batteries.setMatrixAt(index, matrix.matrix);
    batteries.setColorAt(
      index,
      idle.clone().lerp(teal, 0.35 + 0.65 * strength(place.cell)),
    );
  }
  const group = new THREE.Group();
  group.add(homes, batteries);
  return {
    group,
    dispose() {
      homes.material.dispose();
      homes.dispose();
      batteries.material.dispose();
      batteries.dispose();
    },
  };
}

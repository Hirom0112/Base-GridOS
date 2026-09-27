import * as THREE from "three";
import { fieldGround, type GridCell } from "./scene";
import groundImagery from "./austin-ground.jpg";

export function buildGround(cells: GridCell[], loaded: () => void) {
  const ground = fieldGround(cells);
  const group = new THREE.Group();
  const lattice: number[] = [];
  const shade: number[] = [];
  const base = new THREE.Color(0x24365e);
  for (const boundary of ground.lattice)
    for (const [index, [x, z]] of boundary.entries()) {
      const [nx = x, nz = z] = boundary[(index + 1) % boundary.length] ?? [];
      lattice.push(x, -0.01, z, nx, -0.01, nz);
      for (const [px, pz] of [
        [x, z],
        [nx, nz],
      ]) {
        const fade = Math.max(
          0,
          1 - Math.hypot(px ?? 0, pz ?? 0) / (ground.radius || 1),
        );
        const tone = base.clone().multiplyScalar(fade ** 1.4);
        shade.push(tone.r, tone.g, tone.b);
      }
    }
  const latticeLines = new THREE.LineSegments(
    lineGeometry(lattice, shade),
    new THREE.LineBasicMaterial({ vertexColors: true, depthWrite: false }),
  );
  const glow = ground.outline.map((ring) => outlineRibbon(ring));
  const imagery = imageryPlane(ground.imagery, loaded);
  group.add(imagery, latticeLines, ...glow);
  return {
    group,
    places: ground.places,
    dispose() {
      latticeLines.geometry.dispose();
      latticeLines.material.dispose();
      imagery.geometry.dispose();
      imagery.material.map?.dispose();
      imagery.material.dispose();
      for (const ribbon of glow)
        ribbon.traverse((object) => {
          if (object instanceof THREE.Mesh || object instanceof THREE.Line) {
            object.geometry.dispose();
            (object.material as THREE.Material).dispose();
          }
        });
    },
  };
}

function outlineRibbon(ring: [number, number][]) {
  const group = new THREE.Group();
  for (const [width, opacity] of [
    [1.6, 0.1],
    [0.55, 0.3],
  ] as const) {
    const vertices = ring.flatMap(([x, z], index) => {
      const [nx = x, nz = z] = ring[(index + 1) % ring.length] ?? [];
      const length = Math.hypot(nx - x, nz - z) || 1;
      const ox = (-(nz - z) / length) * (width / 2);
      const oz = ((nx - x) / length) * (width / 2);
      return [
        [x - ox, z - oz],
        [nx - ox, nz - oz],
        [nx + ox, nz + oz],
        [x - ox, z - oz],
        [nx + ox, nz + oz],
        [x + ox, z + oz],
      ].flatMap(([px, pz]) => [px ?? 0, 0.02, pz ?? 0]);
    });
    group.add(
      new THREE.Mesh(
        lineGeometry(vertices),
        new THREE.MeshBasicMaterial({
          color: 0x4ff0d2,
          transparent: true,
          opacity,
          blending: THREE.AdditiveBlending,
          depthWrite: false,
          side: THREE.DoubleSide,
        }),
      ),
    );
  }
  const closed = [...ring, ring[0] ?? [0, 0]].flatMap(([x, z]) => [x, 0.03, z]);
  group.add(
    new THREE.Line(
      lineGeometry(closed),
      new THREE.LineBasicMaterial({ color: 0x9ff8ea }),
    ),
  );
  return group;
}

function imageryPlane(
  bounds: { west: number; east: number; north: number; south: number },
  loaded: () => void,
) {
  const texture = new THREE.TextureLoader().load(groundImagery, loaded);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.anisotropy = 4;
  const plane = new THREE.Mesh(
    new THREE.PlaneGeometry(
      bounds.east - bounds.west,
      bounds.south - bounds.north,
    ),
    new THREE.MeshBasicMaterial({
      map: texture,
      depthWrite: false,
      color: 0x9aa4b8,
    }),
  );
  plane.rotation.x = -Math.PI / 2;
  plane.position.set(
    (bounds.east + bounds.west) / 2,
    -0.05,
    (bounds.south + bounds.north) / 2,
  );
  plane.renderOrder = -1;
  return plane;
}

export function lineGeometry(vertices: number[], colors?: number[]) {
  const geometry = new THREE.BufferGeometry();
  geometry.setAttribute(
    "position",
    new THREE.Float32BufferAttribute(vertices, 3),
  );
  if (colors)
    geometry.setAttribute("color", new THREE.Float32BufferAttribute(colors, 3));
  return geometry;
}

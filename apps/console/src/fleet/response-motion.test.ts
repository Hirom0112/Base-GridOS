import * as THREE from "three";
import { afterEach, expect, test, vi } from "vitest";
import { animateResponse } from "./response-motion";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

test("a response highlight ends at its source color and stops scheduling frames", () => {
  vi.spyOn(performance, "now").mockReturnValue(1000);
  vi.stubGlobal("matchMedia", () => ({ matches: false }));
  const frames: FrameRequestCallback[] = [];
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) =>
    frames.push(callback),
  );
  vi.stubGlobal("cancelAnimationFrame", vi.fn());
  const mesh = new THREE.InstancedMesh(
    new THREE.CylinderGeometry(),
    new THREE.MeshBasicMaterial(),
    1,
  );
  const original = new THREE.Color(0x66f2a4);
  mesh.setColorAt(0, original);
  const draw = vi.fn();
  animateResponse(mesh, [0], draw, () => true);
  frames.shift()!(1300);
  const highlighted = new THREE.Color();
  mesh.getColorAt(0, highlighted);
  expect(highlighted.r).toBeGreaterThan(original.r);
  frames.shift()!(1600);
  mesh.getColorAt(0, highlighted);
  expect(highlighted.r).toBeCloseTo(original.r);
  expect(frames).toHaveLength(0);
  expect(draw).toHaveBeenCalledTimes(2);
  mesh.dispose();
  mesh.geometry.dispose();
});

test("reduced motion schedules no geographic highlight", () => {
  vi.stubGlobal("matchMedia", () => ({ matches: true }));
  const frame = vi.fn();
  vi.stubGlobal("requestAnimationFrame", frame);
  const mesh = new THREE.InstancedMesh(
    new THREE.CylinderGeometry(),
    new THREE.MeshBasicMaterial(),
    1,
  );
  const cancel = animateResponse(mesh, [0], vi.fn(), () => true);
  cancel();
  expect(frame).not.toHaveBeenCalled();
  mesh.dispose();
  mesh.geometry.dispose();
});

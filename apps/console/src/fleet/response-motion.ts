import * as THREE from "three";

export function animateResponse(
  mesh: THREE.InstancedMesh,
  indices: number[],
  render: () => void,
  visible: () => boolean,
) {
  if (!indices.length || matchMedia("(prefers-reduced-motion: reduce)").matches)
    return () => {};
  const colors = indices.map((index) => {
    const color = new THREE.Color();
    mesh.getColorAt(index, color);
    return { index, color };
  });
  let frame = 0;
  const started = performance.now();
  function stop() {
    cancelAnimationFrame(frame);
    for (const { index, color } of colors) mesh.setColorAt(index, color);
    if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
    render();
  }
  function tick(now: number) {
    const elapsed = now - started;
    if (
      elapsed >= 600 ||
      !visible() ||
      document.hidden ||
      matchMedia("(prefers-reduced-motion: reduce)").matches
    )
      return stop();
    const glow = Math.sin((elapsed / 600) * Math.PI) * 0.35;
    for (const { index, color } of colors)
      mesh.setColorAt(
        index,
        color.clone().lerp(new THREE.Color(0xffffff), glow),
      );
    if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
    render();
    frame = requestAnimationFrame(tick);
  }
  frame = requestAnimationFrame(tick);
  return stop;
}

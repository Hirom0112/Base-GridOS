export function createRenderQuality(devicePixelRatio: number, width: number) {
  let pixelRatio = Math.min(devicePixelRatio, width < 768 ? 1 : 1.5);
  const budget = 1000 / (width < 768 ? 30 : 55);
  let previous: number | undefined;
  let elapsed = 0;
  let frames = 0;
  return {
    get pixelRatio() {
      return pixelRatio;
    },
    frame(at?: number) {
      if (at === undefined) {
        previous = undefined;
        return pixelRatio;
      }
      if (previous !== undefined) {
        elapsed += at - previous;
        frames++;
      }
      previous = at;
      if (frames < 12) return pixelRatio;
      if (elapsed / frames > budget)
        pixelRatio = Math.min(pixelRatio, pixelRatio > 1 ? 1 : 0.75);
      elapsed = 0;
      frames = 0;
      return pixelRatio;
    },
  };
}

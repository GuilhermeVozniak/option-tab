export interface MagnificationSpring {
  scale: number;
  velocity: number;
}
export function springStep(
  s: MagnificationSpring,
  target: number,
  elapsed: number,
  max: number,
): MagnificationSpring {
  const dt = Math.max(0, Math.min(0.25, elapsed)),
    omega = 20;
  const y = s.scale - target,
    c = s.velocity + omega * y,
    e = Math.exp(-omega * dt);
  const raw = target + (y + c * dt) * e;
  const scale = Math.max(1, Math.min(max, raw));
  const velocity = scale !== raw ? 0 : (s.velocity - omega * c * dt) * e;
  return Math.abs(scale - target) < 0.0005 && Math.abs(velocity) < 0.005
    ? { scale: target, velocity: 0 }
    : { scale, velocity };
}
export function magnificationTargets(
  centers: number[],
  pointer: number | null,
  max: number,
  reach: number,
): number[] {
  if (pointer === null || !centers.length) return centers.map(() => 1);
  let nearest = 0;
  for (let i = 1; i < centers.length; i++)
    if (Math.abs(centers[i] - pointer) < Math.abs(centers[nearest] - pointer)) nearest = i;
  const pitch =
    centers.length > 1
      ? nearest === centers.length - 1
        ? centers[nearest] - centers[nearest - 1]
        : centers[nearest + 1] - centers[nearest]
      : 40;
  const coordinate = nearest + (pointer - centers[nearest]) / Math.max(1, pitch),
    radius = reach + 0.5;
  return centers.map((_, i) => {
    const distance = Math.abs(i - coordinate);
    return distance >= radius
      ? 1
      : 1 + (max - 1) * 0.5 * (1 + Math.cos((Math.PI * distance) / radius));
  });
}
// The transient total is bounded even when the pointer moves faster than old
// springs settle. Centered cumulative expansion keeps every visual gap intact.
export function magnificationTransforms(
  scales: number[],
  icon: number,
  max: number,
  reach: number,
) {
  const extra = scales.map((s) => Math.max(0, Math.min(max - 1, s - 1)) * icon);
  const total = extra.reduce((a, b) => a + b, 0),
    cap = icon * (max - 1) * (2 * reach + 1),
    factor = total > cap ? cap / total : 1;
  const bounded = total * factor;
  let before = 0;
  return extra.map((value) => {
    const width = value * factor;
    const result = { scale: 1 + width / icon, shift: before + width / 2 - bounded / 2 };
    before += width;
    return result;
  });
}

export function fitMagnification(
  transforms: Array<{ scale: number; shift: number }>,
  boxes: Array<{ start: number; end: number }>,
  viewport: { start: number; end: number },
) {
  if (viewport.end <= viewport.start || boxes.length !== transforms.length) return transforms;
  let factor = 1;
  boxes.forEach((box, i) => {
    if (box.start < viewport.start || box.end > viewport.end) {
      // Keep ordinary scroll clipping at its baseline: a partly visible icon
      // cannot safely participate in centered displacement or expansion.
      if (
        box.end > viewport.start &&
        box.start < viewport.end &&
        (transforms[i].scale > 1 || transforms[i].shift !== 0)
      )
        factor = 0;
      return;
    }
    const extra = ((box.end - box.start) * (transforms[i].scale - 1)) / 2;
    const start = transforms[i].shift - extra,
      end = transforms[i].shift + extra;
    if (start < 0) factor = Math.min(factor, (box.start - viewport.start) / -start);
    if (end > 0) factor = Math.min(factor, (viewport.end - box.end) / end);
  });
  return transforms.map((t) => ({
    scale: 1 + (t.scale - 1) * factor,
    shift: factor === 0 ? 0 : t.shift * factor,
  }));
}

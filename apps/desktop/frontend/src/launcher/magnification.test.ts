import { expect, it } from "vitest";
import {
  fitMagnification,
  magnificationTargets,
  magnificationTransforms,
  springStep,
} from "./magnification";

it("uses elapsed time consistently at60 and120Hz and settles without overshoot", () => {
  const run = (hz: number) => {
    let s = { scale: 1, velocity: 0 };
    for (let i = 0; i < hz; i++) {
      s = springStep(s, 2, 1 / hz, 2);
      expect(s.scale).toBeGreaterThanOrEqual(1);
      expect(s.scale).toBeLessThanOrEqual(2);
    }
    return s;
  };
  expect(run(60).scale).toBeCloseTo(run(120).scale, 6);
  expect(run(120).scale).toBeCloseTo(2, 3);
});
it("returns to rest and bounds target reach without hover actions", () => {
  expect(magnificationTargets([20, 70, 120], null, 2, 2)).toEqual([1, 1, 1]);
  expect(magnificationTargets([20, 70, 120], 70, 2, 0)).toEqual([1, 2, 1]);
  let s = { scale: 2, velocity: 0 };
  for (let i = 0; i < 240; i++) s = springStep(s, 1, 1 / 120, 2);
  expect(s).toEqual({ scale: 1, velocity: 0 });
});
it("clamps lingering spring expansion and preserves nonoverlapping visual order", () => {
  const states = Array.from({ length: 20 }, () => 2);
  const result = magnificationTransforms(states, 40, 2, 1);
  expect(result.reduce((sum, x) => sum + 40 * (x.scale - 1), 0)).toBeLessThanOrEqual(120 + 1e-8);
  for (let i = 1; i < result.length; i++) {
    const prior = (i - 1) * 46 + result[i - 1].shift + 20 * result[i - 1].scale;
    const next = i * 46 + result[i].shift - 20 * result[i].scale;
    expect(next - prior).toBeCloseTo(6, 6);
  }
  expect(result[0].shift - 20 * (result[0].scale - 1)).toBeGreaterThanOrEqual(-60 - 1e-8);
  expect(result.at(-1)!.shift + 20 * (result.at(-1)!.scale - 1)).toBeLessThanOrEqual(60 + 1e-8);
});

it("reduces expansion at a scrolled viewport edge without changing relative gaps", () => {
  const positions = [
    { start: -25, end: 15 },
    { start: 21, end: 61 },
    { start: 67, end: 107 },
  ];
  const transformed = magnificationTransforms([1, 2, 1.5], 40, 2, 1);
  const adjusted = fitMagnification(transformed, positions, { start: 0, end: 115 });
  for (let i = 1; i < 3; i++) {
    expect(
      positions[i].start + adjusted[i].shift - 20 * (adjusted[i].scale - 1),
    ).toBeGreaterThanOrEqual(0);
    expect(positions[i].end + adjusted[i].shift + 20 * (adjusted[i].scale - 1)).toBeLessThanOrEqual(
      115,
    );
  }
});

it("does not magnify or displace the actual partially visible first icon", () => {
  const boxes = [
    { start: -25, end: 15 },
    { start: 21, end: 61 },
    { start: 67, end: 107 },
  ];
  const result = fitMagnification(magnificationTransforms([2, 1.8, 1], 40, 2, 1), boxes, {
    start: 0,
    end: 115,
  });
  expect(result[0]).toEqual({ scale: 1, shift: 0 });
  expect(result).toEqual(boxes.map(() => ({ scale: 1, shift: 0 })));
});

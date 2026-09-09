import { act, fireEvent, render } from "@testing-library/react";
import { useRef } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { useMagnification } from "./useMagnification";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});
function Fixture({
  enabled = true,
  owner = "one",
  revision = 1,
}: {
  enabled?: boolean;
  owner?: string;
  revision?: number;
}) {
  const ref = useRef<HTMLUListElement>(null);
  useMagnification(ref, {
    enabled,
    owner,
    itemsKey: "a",
    scale: 2,
    reach: 1,
    iconPx: 40,
    vertical: false,
  });
  return (
    <ul ref={ref} data-revision={revision}>
      <li>
        <button className="ot-launcher-app" type="button">
          <span className="ot-launcher-visual">A</span>
        </button>
      </li>
    </ul>
  );
}
function scheduler() {
  vi.stubGlobal("PointerEvent", MouseEvent);
  let now = 0,
    id = 0;
  const q = new Map<number, FrameRequestCallback>();
  vi.stubGlobal("requestAnimationFrame", (f: FrameRequestCallback) => {
    q.set(++id, f);
    return id;
  });
  vi.stubGlobal("cancelAnimationFrame", (n: number) => q.delete(n));
  return {
    q,
    tick() {
      now += 1000 / 120;
      const callbacks = [...q.values()];
      q.clear();
      act(() => callbacks.forEach((f) => f(now)));
    },
  };
}
it("runs one elapsed-time loop, preserves clock ownership and stops on leave/unmount", () => {
  const clock = scheduler();
  vi.stubGlobal("matchMedia", () => ({
    matches: false,
    addEventListener() {},
    removeEventListener() {},
  }));
  const { container, rerender, unmount } = render(<Fixture />);
  const list = container.querySelector("ul")!,
    button = container.querySelector("button")!,
    visual = container.querySelector("span")!;
  button.getBoundingClientRect = () =>
    ({ left: 0, top: 0, right: 40, bottom: 40, width: 40, height: 40 }) as DOMRect;
  fireEvent.pointerMove(list, { clientX: 20, clientY: 20 });
  expect(clock.q.size).toBe(1);
  for (let i = 0; i < 120; i++) clock.tick();
  expect(visual.style.transform).toContain("scale(2)");
  expect(clock.q.size).toBe(0);
  rerender(<Fixture revision={2} />);
  expect(visual.style.transform).toContain("scale(2)");
  fireEvent.pointerLeave(list);
  for (let i = 0; i < 180; i++) clock.tick();
  expect(visual.style.transform).toContain("scale(1)");
  expect(clock.q.size).toBe(0);
  fireEvent.pointerMove(list, { clientX: 20 });
  unmount();
  expect(clock.q.size).toBe(0);
});
it("reduced motion, disabled and retired owners immediately return to scale1", () => {
  const clock = scheduler();
  let reduced = false;
  let notify = () => {};
  vi.stubGlobal("matchMedia", () => ({
    get matches() {
      return reduced;
    },
    addEventListener(_type: string, f: () => void) {
      notify = f;
    },
    removeEventListener() {},
  }));
  const { container, rerender } = render(<Fixture />);
  const list = container.querySelector("ul")!,
    button = container.querySelector("button")!,
    visual = container.querySelector("span")!;
  button.getBoundingClientRect = () =>
    ({ left: 0, top: 0, right: 40, bottom: 40, width: 40, height: 40 }) as DOMRect;
  fireEvent.pointerMove(list, { clientX: 20 });
  clock.tick();
  clock.tick();
  act(() => {
    reduced = true;
    notify();
  });
  expect(visual.style.transform).toContain("scale(1)");
  expect(clock.q.size).toBe(0);
  rerender(<Fixture enabled={false} owner="two" />);
  fireEvent.pointerMove(list, { clientX: 20 });
  expect(clock.q.size).toBe(0);
});

it("synchronously stops for document hiding and owner retirement", () => {
  const clock = scheduler();
  vi.stubGlobal("matchMedia", () => ({
    matches: false,
    addEventListener() {},
    removeEventListener() {},
  }));
  const { container, rerender } = render(<Fixture />);
  const list = container.querySelector("ul")!,
    button = container.querySelector("button")!,
    visual = container.querySelector("span")!;
  button.getBoundingClientRect = () =>
    ({ left: 0, top: 0, right: 40, bottom: 40, width: 40, height: 40 }) as DOMRect;
  fireEvent.pointerMove(list, { clientX: 20 });
  clock.tick();
  clock.tick();
  expect(clock.q.size).toBe(1);
  rerender(<Fixture owner="replacement" />);
  expect(clock.q.size).toBe(0);
  expect(visual.style.transform).toContain("scale(1)");
  fireEvent.pointerMove(list, { clientX: 20 });
  clock.tick();
  clock.tick();
  vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
  act(() => document.dispatchEvent(new Event("visibilitychange")));
  expect(clock.q.size).toBe(0);
  expect(visual.style.transform).toContain("scale(1)");
});

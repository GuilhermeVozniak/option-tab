import { act, render } from "@testing-library/react";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  admitMaterialStatus,
  type MaterialRect,
  materialClass,
  useMaterialReporter,
} from "./material";

let resize: () => void = () => {};
class Observer {
  constructor(callback: () => void) {
    resize = callback;
  }
  observe() {}
  disconnect() {}
}

function Panel({ revision, report }: { revision: number; report: (rect: MaterialRect) => void }) {
  const ref = useMaterialReporter(7, revision, report);
  return <section ref={ref}>panel</section>;
}

describe("material frontend admission", () => {
  beforeEach(() => {
    vi.stubGlobal("ResizeObserver", Observer);
    vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
      callback(0);
      return 1;
    });
    vi.stubGlobal("cancelAnimationFrame", () => {});
  });

  it("uses truthful solid fallback until matching native system status", () => {
    expect(materialClass(true, null)).toBe("ot-solid-material");
    expect(materialClass(true, { session: 7, revision: 1, state: "unavailable" })).toBe(
      "ot-solid-material",
    );
    expect(materialClass(true, { session: 7, revision: 2, state: "system" })).toBe(
      "ot-native-material",
    );
    expect(materialClass(false, { session: 7, revision: 2, state: "system" })).toBe(
      "ot-solid-material",
    );
  });

  it("rejects wrong-session, zero, equal, and reordered material revisions", () => {
    const first = { session: 7, revision: 4, state: "system" as const };
    expect(admitMaterialStatus(null, { ...first, session: 6 }, 7)).toBeNull();
    expect(admitMaterialStatus(null, { ...first, revision: 0 }, 7)).toBeNull();
    expect(admitMaterialStatus(first, { ...first, revision: 3 }, 7)).toBe(first);
    expect(admitMaterialStatus(first, { ...first, state: "solid" }, 7)).toBe(first);
    expect(admitMaterialStatus(first, { ...first, revision: 5, state: "solid" }, 7)?.state).toBe(
      "solid",
    );
  });

  it("reports the exact panel rect again for a new state revision and suppresses duplicates", () => {
    vi.useFakeTimers();
    const reports: MaterialRect[] = [];
    const { container, rerender, unmount } = render(
      <Panel revision={2} report={(r) => reports.push(r)} />,
    );
    const panel = container.querySelector("section") as HTMLElement;
    vi.spyOn(panel, "getBoundingClientRect").mockReturnValue({
      x: 18,
      y: 24,
      width: 320,
      height: 180,
      top: 24,
      left: 18,
      right: 338,
      bottom: 204,
      toJSON: () => ({}),
    });
    act(() => vi.runAllTimers());
    expect(reports.at(-1)).toMatchObject({ session: 7, stateRevision: 2, x: 18, width: 320 });
    act(() => resize());
    act(() => vi.runAllTimers());
    expect(reports).toHaveLength(1);
    rerender(<Panel revision={3} report={(r) => reports.push(r)} />);
    act(() => vi.runAllTimers());
    expect(reports.at(-1)).toMatchObject({ stateRevision: 3, sequence: 2 });
    unmount();
    act(() => resize());
    act(() => vi.runAllTimers());
    expect(reports).toHaveLength(2);
    vi.useRealTimers();
  });
});

import { act, fireEvent, render } from "@testing-library/react";
import { useRef } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { useLauncherReorder } from "./useLauncherReorder";

afterEach(() => vi.unstubAllGlobals());
const items = [
  { id: "pin:a", name: "A", kind: "app", icon: "" },
  { id: "pin:b", name: "B", kind: "app", icon: "" },
];
function Fixture({
  owner = "1",
  enabled = true,
  perform = vi.fn(),
}: {
  owner?: string;
  enabled?: boolean;
  perform?: (m: unknown) => void;
}) {
  const ref = useRef<HTMLUListElement>(null);
  const drag = useLauncherReorder(ref, { enabled, owner, items, vertical: false, perform });
  return (
    <ul ref={ref} onClickCapture={drag.suppressClick}>
      <li>
        <button data-reorder-target="pin:a" onPointerDown={(e) => drag.start(e, "pin:a")}>
          A
        </button>
      </li>
      <li>
        <button data-reorder-target="pin:b">B</button>
      </li>
      <output>{drag.target?.kind}</output>
    </ul>
  );
}
function setup() {
  vi.stubGlobal("PointerEvent", MouseEvent);
  const perform = vi.fn(),
    view = render(<Fixture perform={perform} />);
  const buttons = view.container.querySelectorAll("button");
  buttons.forEach((b, i) => {
    b.getBoundingClientRect = () =>
      ({ left: i * 50, right: i * 50 + 40, top: 0, bottom: 40, width: 40, height: 40 }) as DOMRect;
  });
  return { ...view, buttons, perform };
}
it("requires threshold and dispatches one internal exact mutation with click suppression", () => {
  const { buttons, perform, container } = setup();
  fireEvent.pointerDown(buttons[0], { button: 0, clientX: 20, clientY: 20 });
  fireEvent.pointerMove(window, { clientX: 24, clientY: 20 });
  expect(container.querySelector("output")!.textContent).toBe("");
  fireEvent.pointerMove(window, { clientX: 87, clientY: 20 });
  fireEvent.pointerUp(window, { clientX: 87, clientY: 20 });
  expect(perform).toHaveBeenCalledExactlyOnceWith({
    kind: "moveAfter",
    itemID: "pin:a",
    targetID: "pin:b",
  });
  const click = new MouseEvent("click", { bubbles: true, cancelable: true });
  act(() => buttons[0].dispatchEvent(click));
  expect(click.defaultPrevented).toBe(true);
});
it("cancels on revision retirement/escape and ignores external drags", () => {
  const { buttons, perform, rerender } = setup();
  fireEvent.pointerDown(buttons[0], { button: 0, clientX: 20, clientY: 20 });
  fireEvent.pointerMove(window, { clientX: 87, clientY: 20 });
  rerender(<Fixture owner="2" perform={perform} />);
  fireEvent.pointerUp(window, { clientX: 87, clientY: 20 });
  expect(perform).not.toHaveBeenCalled();
  fireEvent.drop(buttons[1], { dataTransfer: { getData: () => "pin:a" } });
  expect(perform).not.toHaveBeenCalled();
});

it("escape and pointer cancellation prevent a queued drag action", () => {
  const { buttons, perform } = setup();
  for (const cancel of [
    () => fireEvent.keyDown(window, { key: "Escape" }),
    () => fireEvent.pointerCancel(window),
  ]) {
    fireEvent.pointerDown(buttons[0], { button: 0, clientX: 20, clientY: 20 });
    fireEvent.pointerMove(window, { clientX: 87, clientY: 20 });
    cancel();
    fireEvent.pointerUp(window, { clientX: 87, clientY: 20 });
  }
  expect(perform).not.toHaveBeenCalled();
});

it("rechecks release position so dropping outside cannot use a previous target", () => {
  const { buttons, perform } = setup();
  fireEvent.pointerDown(buttons[0], { button: 0, clientX: 20, clientY: 20 });
  fireEvent.pointerMove(window, { clientX: 87, clientY: 20 });
  fireEvent.pointerUp(window, { clientX: 400, clientY: 400 });
  expect(perform).not.toHaveBeenCalled();
});

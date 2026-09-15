import { act, fireEvent, render } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { LauncherPresentation } from "../lib/types";
import { useLauncherAutoHideHold } from "./useLauncherAutoHideHold";

const p = {
  epoch: 1,
  session: 2,
  displayUUID: "display",
  revision: 3,
  profileID: "profile",
  visible: true,
} as LauncherPresentation;
const send = vi.fn().mockResolvedValue(undefined);
function Fixture({
  presentation = p,
  menu = false,
}: {
  presentation?: LauncherPresentation;
  menu?: boolean;
}) {
  const hold = useLauncherAutoHideHold(
    presentation,
    menu,
    send,
    () => {},
    JSON.stringify(presentation.items),
  );
  return <button {...hold}>Item</button>;
}
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  send.mockClear();
});

it("keeps an owned pointer held outside the panel and releases on up or cancel", async () => {
  vi.useFakeTimers();
  vi.stubGlobal("PointerEvent", MouseEvent);
  const view = render(<Fixture />);
  fireEvent.pointerDown(view.getByRole("button"), { button: 0 });
  expect(send).toHaveBeenLastCalledWith(1, "display", 2, 3, 1, "begin");
  fireEvent.pointerMove(window, { clientX: -500, clientY: -500 });
  act(() => vi.advanceTimersByTime(600));
  await act(async () => {});
  expect(send.mock.calls.at(-1)?.at(-1)).toBe("renew");
  fireEvent.pointerUp(window);
  await act(async () => {});
  expect(send.mock.calls.at(-1)?.at(-1)).toBe("end");
  const count = send.mock.calls.length;
  act(() => vi.advanceTimersByTime(2100));
  expect(send).toHaveBeenCalledTimes(count);
  fireEvent.pointerDown(view.getByRole("button"), { button: 0 });
  fireEvent.pointerCancel(window);
  await act(async () => {});
  expect(send.mock.calls.at(-1)?.at(-1)).toBe("end");
});

it("renews menus with the latest clock revision and releases old owners on retirement", async () => {
  vi.useFakeTimers();
  const view = render(<Fixture menu />);
  view.rerender(<Fixture menu presentation={{ ...p, revision: 4 }} />);
  act(() => vi.advanceTimersByTime(600));
  await act(async () => {});
  expect(send.mock.calls.at(-1)?.slice(0, 4)).toEqual([1, "display", 2, 4]);
  expect(send.mock.calls.at(-1)?.at(-1)).toBe("renew");
  view.rerender(<Fixture presentation={{ ...p, epoch: 2, session: 5 }} />);
  await act(async () => {});
  expect(send.mock.calls.at(-1)?.slice(0, 4)).toEqual([1, "display", 2, 4]);
  expect(send.mock.calls.at(-1)?.at(-1)).toBe("end");
  const count = send.mock.calls.length;
  act(() => vi.advanceTimersByTime(2100));
  expect(send).toHaveBeenCalledTimes(count);
});

it("releases on unmount, Escape and blur without an unhandled bridge rejection", async () => {
  vi.useFakeTimers();
  vi.stubGlobal("PointerEvent", MouseEvent);
  send.mockRejectedValueOnce(new Error("retired"));
  const view = render(<Fixture />);
  fireEvent.pointerDown(view.getByRole("button"), { button: 0 });
  fireEvent.keyDown(window, { key: "Escape" });
  await act(async () => {});
  expect(send.mock.calls.at(-1)?.at(-1)).toBe("end");
  fireEvent.pointerDown(view.getByRole("button"), { button: 0 });
  fireEvent.blur(window);
  await act(async () => {});
  expect(send.mock.calls.at(-1)?.at(-1)).toBe("end");
  fireEvent.pointerDown(view.getByRole("button"), { button: 0 });
  view.unmount();
  await act(async () => {});
  expect(send.mock.calls.at(-1)?.at(-1)).toBe("end");
  await act(async () => {});
});

it("bounds pending requests and coalesces renewals into cancellation when the bridge stalls", async () => {
  vi.useFakeTimers();
  let resolve!: () => void;
  send.mockImplementationOnce(
    () =>
      new Promise<void>((done) => {
        resolve = done;
      }),
  );
  const view = render(<Fixture menu />);
  act(() => vi.advanceTimersByTime(30_000));
  expect(send).toHaveBeenCalledTimes(1);
  view.rerender(<Fixture />);
  await act(async () => resolve());
  expect(send).toHaveBeenCalledTimes(2);
  expect(send.mock.calls[1]).toEqual([1, "display", 2, 3, 2, "end"]);
});

it("retires held input on item or geometry changes and admits only a fresh pointer", async () => {
  vi.useFakeTimers();
  vi.stubGlobal("PointerEvent", MouseEvent);
  const view = render(<Fixture />);
  fireEvent.pointerDown(view.getByRole("button"), { button: 0 });
  const next = { ...p, revision: 4, items: [{ id: "new", name: "New", icon: "" }] };
  view.rerender(<Fixture presentation={next} />);
  await act(async () => {});
  expect(send.mock.calls.at(-1)?.at(-1)).toBe("end");
  const count = send.mock.calls.length;
  await act(async () => vi.advanceTimersByTime(2100));
  expect(send).toHaveBeenCalledTimes(count);
  fireEvent.pointerDown(view.getByRole("button"), { button: 0 });
  await act(async () => {});
  expect(send.mock.calls.at(-1)?.slice(0, 4)).toEqual([1, "display", 2, 4]);
  expect(send.mock.calls.at(-1)?.at(-1)).toBe("begin");
  view.rerender(
    <Fixture presentation={{ ...next, revision: 5, bounds: { x: 1, y: 1, w: 50, h: 50 } }} />,
  );
  await act(async () => {});
  expect(send.mock.calls.at(-1)?.at(-1)).toBe("end");
});

it("preserves a queued fresh begin while newer renewals are coalesced", async () => {
  vi.useFakeTimers();
  let resolve!: () => void;
  send.mockImplementationOnce(
    () =>
      new Promise<void>((done) => {
        resolve = done;
      }),
  );
  const view = render(<Fixture menu />);
  view.rerender(<Fixture />);
  view.rerender(<Fixture menu />);
  act(() => vi.advanceTimersByTime(600));
  await act(async () => resolve());
  expect(send).toHaveBeenCalledTimes(2);
  expect(send.mock.calls[1].at(-1)).toBe("begin");
});

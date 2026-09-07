import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import type {
  LauncherItemPanelState,
  LauncherItemPanelTransport,
} from "../lib/launcher-item-panel-bridge";
import { emptyState } from "../lib/types";
import { LauncherItemPanelRoute } from "./LauncherItemPanelRoute";

const state = (revision = 1, view: "list" | "grid" = "list"): LauncherItemPanelState => ({
  session: 8,
  revision,
  parentEpoch: 2,
  parentSession: 3,
  displayUUID: "main",
  profileID: "default",
  itemID: "docs",
  kind: "folder",
  title: "Documents",
  open: true,
  bounds: { x: 0, y: 0, w: 420, h: 360 },
  folder: {
    status: "ready",
    reason: "",
    folderIdentity: "launcher-child:8",
    view,
    entries: [
      {
        id: "opaque",
        name: "Quarterly notes",
        kind: "file",
        size: 12,
        modifiedAtMs: 1,
        hidden: false,
      },
    ],
    sort: { field: "name", direction: "asc", foldersFirst: true },
    partial: false,
    revision,
  },
});

const windowsState = (revision = 1): LauncherItemPanelState => ({
  ...state(revision),
  kind: "windows",
  folder: undefined,
  windows: {
    open: true,
    session: 8,
    revision,
    title: "Editor windows",
    entries: [
      {
        windowId: 44,
        appId: 9,
        appName: "Editor",
        bundleId: "com.example.editor",
        title: "Plan",
        spaceId: 1,
        minimized: false,
        hidden: false,
        fullscreen: false,
      },
    ],
    selectedWindowId: 44,
    appearance: { ...emptyState.appearance, blur: false, apparitionDelayMs: 0 },
    cardSpacingPx: 7,
    emptyReason: "",
    frames: {},
    frameSequence: 0,
  },
});

function transport(snapshot: LauncherItemPanelState | null = state()) {
  let handlers!: Parameters<LauncherItemPanelTransport["subscribe"]>[0];
  const api: LauncherItemPanelTransport = {
    getState: vi.fn().mockResolvedValue(snapshot),
    close: vi.fn().mockResolvedValue(undefined),
    size: vi.fn().mockResolvedValue(undefined),
    sort: vi.fn().mockResolvedValue(undefined),
    view: vi.fn().mockResolvedValue(undefined),
    open: vi.fn().mockResolvedValue(undefined),
    subscribe: vi.fn((next) => {
      handlers = next;
      return vi.fn();
    }),
  };
  return { api, emit: () => handlers };
}

it("renders list and grid with exact child scope actions", async () => {
  const t = transport();
  render(<LauncherItemPanelRoute session={8} transport={t.api} />);
  fireEvent.click(await screen.findByRole("button", { name: "Grid" }));
  expect(t.api.view).toHaveBeenCalledWith(8, 1, "grid");
  await act(async () => t.emit().update(state(2, "grid")));
  expect(document.querySelector(".ot-folder-list")).toHaveClass("is-grid");
  fireEvent.click(screen.getByRole("button", { name: "Open Quarterly notes" }));
  expect(t.api.open).toHaveBeenCalledWith(8, 2, "opaque");
});

it("rejects stale snapshots and terminally tombstones late updates", async () => {
  let resolve!: (value: LauncherItemPanelState) => void;
  const t = transport(null);
  vi.mocked(t.api.getState).mockReturnValue(new Promise((done) => (resolve = done)));
  render(<LauncherItemPanelRoute session={8} transport={t.api} />);
  await act(async () => t.emit().update(state(4)));
  expect(screen.getByText("Documents")).toBeVisible();
  await act(async () => t.emit().hide({ session: 8, revision: 5 }));
  await act(async () => resolve(state(3)));
  await act(async () => t.emit().update(state(6)));
  expect(screen.queryByText("Documents")).toBeNull();
});

it("preserves a same-session tombstone when the transport object is replaced", async () => {
  const first = transport(state(2));
  const { rerender } = render(<LauncherItemPanelRoute session={8} transport={first.api} />);
  await screen.findByText("Documents");
  await act(async () => first.emit().hide({ session: 8, revision: 3 }));
  const replacement = transport(state(4));
  rerender(<LauncherItemPanelRoute session={8} transport={replacement.api} />);
  await act(async () => {});
  expect(screen.queryByText("Documents")).toBeNull();
});

it("does not surface an action failure after a newer revision", async () => {
  let reject!: (reason: Error) => void;
  const t = transport();
  vi.mocked(t.api.view).mockReturnValue(new Promise((_, fail) => (reject = fail)));
  render(<LauncherItemPanelRoute session={8} transport={t.api} />);
  fireEvent.click(await screen.findByRole("button", { name: "Grid" }));
  await act(async () => t.emit().update(state(2)));
  await act(async () => reject(new Error("old refusal")));
  expect(screen.queryByRole("alert")).toBeNull();
});

it("measures bounded complete content and closes the admitted revision", async () => {
  const t = transport();
  render(<LauncherItemPanelRoute session={8} transport={t.api} />);
  await screen.findByText("Documents");
  await waitFor(() => expect(t.api.size).toHaveBeenCalledWith(8, 1, 120, 96));
  fireEvent.click(screen.getByRole("button", { name: "Close preview" }));
  expect(t.api.close).toHaveBeenCalledWith(8, 1);
});

it("admits only current monotonic window frames and exact actions", async () => {
  const t = transport(windowsState());
  t.api.selectWindow = vi.fn().mockResolvedValue(undefined);
  t.api.windowAction = vi.fn().mockResolvedValue(undefined);
  const { container } = render(<LauncherItemPanelRoute session={8} transport={t.api} />);
  await screen.findByText("Plan");
  await act(async () =>
    t.emit().frames({
      session: 8,
      revision: 1,
      sequence: 2,
      frames: { "44": "data:image/png;base64,NEW" },
    }),
  );
  expect(container.querySelector("img")).toHaveAttribute("src", "data:image/png;base64,NEW");
  await act(async () =>
    t.emit().frames({
      session: 8,
      revision: 1,
      sequence: 1,
      frames: { "44": "data:image/png;base64,OLD" },
    }),
  );
  expect(container.querySelector("img")).toHaveAttribute("src", "data:image/png;base64,NEW");
  fireEvent.click(screen.getByRole("button", { name: /Plan/ }));
  expect(t.api.windowAction).toHaveBeenCalledWith(8, 1, "focus", 44, false);
});

it("hides only the selected rendered window and displays current RPC refusal", async () => {
  const t = transport(windowsState());
  t.api.windowAction = vi.fn().mockRejectedValue(new Error("Access refused"));
  render(<LauncherItemPanelRoute session={8} transport={t.api} />);
  fireEvent.click(await screen.findByRole("button", { name: "Hide app" }));
  expect(t.api.windowAction).toHaveBeenCalledWith(8, 1, "hide", 44, false);
  expect(await screen.findByRole("alert")).toHaveTextContent("Access refused");
});
it("retains a one-shot frame delivered before its revision update", async () => {
  const t = transport(windowsState());
  const { container } = render(<LauncherItemPanelRoute session={8} transport={t.api} />);
  await screen.findByText("Plan");
  await act(async () =>
    t.emit().frames({
      session: 8,
      revision: 2,
      sequence: 1,
      frames: { "44": "data:image/png;base64,AAAA" },
    }),
  );
  await act(async () => t.emit().update(windowsState(2)));
  expect(container.querySelector("img")).toHaveAttribute("src", "data:image/png;base64,AAAA");
});
it("purges future frames on hide and refuses foreign or oversized snapshot URLs", async () => {
  const t = transport(windowsState());
  const { container } = render(<LauncherItemPanelRoute session={8} transport={t.api} />);
  await screen.findByText("Plan");
  const next = windowsState(2);
  next.windows!.frames = {
    "44": "https://example.invalid/image",
    "99": "data:image/png;base64,AAAA",
  };
  await act(async () => t.emit().update(next));
  expect(container.querySelector("img")).toBeNull();
  await act(async () =>
    t.emit().frames({
      session: 8,
      revision: 3,
      sequence: 3,
      frames: { "44": "data:image/png;base64,AAAA" },
    }),
  );
  await act(async () => t.emit().hide({ session: 8, revision: 3 }));
  await act(async () => t.emit().update(windowsState(3)));
  expect(container.querySelector("img")).toBeNull();
});

it("catches up an initial frame before GetState and a newer same-revision snapshot", async () => {
  const t = transport(null);
  let resolve!: (s: LauncherItemPanelState) => void;
  vi.mocked(t.api.getState).mockReturnValue(
    new Promise((done) => {
      resolve = done;
    }),
  );
  const { container } = render(<LauncherItemPanelRoute session={8} transport={t.api} />);
  await act(async () =>
    t.emit().frames({
      session: 8,
      revision: 1,
      sequence: 2,
      frames: { "44": "data:image/png;base64,AAAA" },
    }),
  );
  await act(async () => resolve(windowsState()));
  expect(container.querySelector("img")).toHaveAttribute("src", "data:image/png;base64,AAAA");
  const newer = windowsState();
  newer.windows!.frameSequence = 3;
  newer.windows!.frames = { "44": "data:image/png;base64,BBBB" };
  await act(async () => t.emit().update(newer));
  expect(container.querySelector("img")).toHaveAttribute("src", "data:image/png;base64,BBBB");
  await act(async () =>
    t.emit().frames({
      session: 8,
      revision: 1,
      sequence: 2,
      frames: { "44": "data:image/png;base64,AAAA" },
    }),
  );
  expect(container.querySelector("img")).toHaveAttribute("src", "data:image/png;base64,BBBB");
});
it("bounds future encoded images and ignores IDs outside the admitted entries", async () => {
  const t = transport(windowsState());
  const { container } = render(<LauncherItemPanelRoute session={8} transport={t.api} />);
  await screen.findByText("Plan");
  await act(async () =>
    t.emit().frames({
      session: 8,
      revision: 2,
      sequence: 1,
      frames: {
        "44": "data:image/png;base64," + "A".repeat(512 * 1024),
        "99": "data:image/png;base64,AAAA",
      },
    }),
  );
  await act(async () => t.emit().update(windowsState(2)));
  expect(container.querySelector("img")).toBeNull();
});

it.each([
  { count: 35, bytes: 4, expected: 30 },
  { count: 9, bytes: 512 * 1024 - 24, expected: 8 },
])("bounds future frame storage to the declared caps: $count entries", async ({
  count,
  bytes,
  expected,
}) => {
  const initial = windowsState();
  initial.windows!.appearance.compactThreshold = 0;
  initial.windows!.entries = Array.from({ length: count }, (_, i) => ({
    ...initial.windows!.entries[0],
    windowId: i + 1,
    title: `Window ${i + 1}`,
  }));
  initial.windows!.selectedWindowId = 1;
  const t = transport(initial);
  const { container } = render(<LauncherItemPanelRoute session={8} transport={t.api} />);
  await screen.findByText("Window 1");
  const value = "data:image/png;base64," + "A".repeat(bytes);
  await act(async () =>
    t.emit().frames({
      session: 8,
      revision: 2,
      sequence: 1,
      frames: Object.fromEntries(initial.windows!.entries.map((e) => [e.windowId, value])),
    }),
  );
  await act(async () =>
    t.emit().update({ ...initial, revision: 2, windows: { ...initial.windows!, revision: 2 } }),
  );
  expect(container.querySelectorAll(".ot-dock-image img")).toHaveLength(expected);
});

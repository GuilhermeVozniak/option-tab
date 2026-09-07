import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { LauncherPresentation } from "../lib/types";
import { LauncherView } from "./LauncherView";
import type { LauncherInteractionTransport } from "./useLauncherInteractions";

const presentation: LauncherPresentation = {
  epoch: 9,
  displayUUID: "display-main",
  session: 12,
  revision: 4,
  visible: true,
  reason: "",
  profileID: "default",
  bounds: { x: 20, y: 700, w: 260, h: 64 },
  iconPx: 40,
  edge: "bottom",
  layout: "floating",
  appearance: {
    theme: "light",
    material: "system",
    tint: "#224466",
    opacity: 0.8,
    borderOpacity: 0.2,
    cornerRadiusPx: 22,
    itemSpacingPx: 9,
    showLabels: false,
  },
  items: [
    { id: "opaque-1", name: "Notes & Tasks", icon: "data:image/png;base64,AA==" },
    { id: "opaque-2", name: "Windowless Helper", icon: "" },
  ],
  widgets: [
    {
      id: "clock",
      packageID: "org.optiontab.clock",
      digest: "builtin-clock-v1",
      status: "ready",
      root: { kind: "row", text: "", children: [{ kind: "text", text: "09:41" }] },
    },
  ],
};

describe("LauncherView", () => {
  it("renders bounded badge counts without changing item activation or inventing unavailable counts", () => {
    const activate = vi.fn();
    const { container, rerender } = render(
      <LauncherView
        presentation={presentation}
        onActivate={activate}
        badges={
          new Map([
            ["opaque-1", { itemID: "opaque-1", state: "known", kind: "count", count: 123 }],
            ["opaque-2", { itemID: "opaque-2", state: "unavailable" }],
          ])
        }
      />,
    );
    const button = screen.getByRole("button", { name: "Notes & Tasks" });
    expect(button).toHaveAttribute("aria-description", "Open application · Dock badge count: 123");
    expect(container.querySelectorAll(".ot-launcher-badge")).toHaveLength(1);
    expect(container.querySelector(".ot-launcher-badge")).toHaveTextContent("99+");
    fireEvent.click(button);
    expect(activate).toHaveBeenCalledWith(9, "display-main", 12, 4, "opaque-1");
    rerender(<LauncherView presentation={presentation} onActivate={activate} badges={new Map()} />);
    expect(container.querySelector(".ot-launcher-badge")).toBeNull();
  });
  it("renders bounded app choices and the trusted clock tree", () => {
    render(<LauncherView presentation={presentation} onActivate={() => {}} />);
    expect(screen.getByRole("button", { name: "Notes & Tasks" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Windowless Helper" })).toBeVisible();
    expect(screen.getByText("09:41")).toBeVisible();
    expect(screen.getByRole("list")).toHaveClass("ot-launcher-strip");
    expect(screen.getByLabelText("Option Tab launcher")).toHaveClass("theme-light", "edge-bottom");
    expect(screen.getByText("Notes & Tasks")).toHaveClass("ot-launcher-label-hidden");
  });

  it("dispatches the exact backend-owned scope and opaque item id", () => {
    const activate = vi.fn();
    render(<LauncherView presentation={presentation} onActivate={activate} />);
    fireEvent.click(screen.getByRole("button", { name: "Windowless Helper" }));
    expect(activate).toHaveBeenCalledWith(9, "display-main", 12, 4, "opaque-2");
  });

  it("does not interpret widget text as markup", () => {
    const hostile = structuredClone(presentation) as any;
    hostile.widgets[0].root.children[0].text = "<img src=x onerror=alert(1)>";
    const { container } = render(<LauncherView presentation={hostile} onActivate={() => {}} />);
    expect(screen.getByText("<img src=x onerror=alert(1)>")).toBeVisible();
    expect(container.querySelector("img[src='x']")).toBeNull();
  });
});

it("requires an explicit keyboard mode and keeps letter selection separate from activation", async () => {
  let publish = (_state: any) => {};
  const state = {
    epoch: 9,
    displayUUID: "display-main",
    session: 12,
    presentationRevision: 4,
    admission: 2,
    sequence: 1,
    visible: true,
    selectedItemID: "opaque-2",
    keyboardMode: false,
    gestureAvailable: true,
    pinchAvailable: false,
    swipeAvailable: false,
    letterInputAvailable: true,
    hapticsAvailable: true,
    configured: {
      enabled: true,
      preciseScroll: true,
      pinch: false,
      swipe: false,
      primaryAction: "next",
      towardAction: "showPreview",
      pinchAction: "showPreview",
      haptics: true,
      letterNavigation: true,
      enterActivates: false,
    },
    reason: "",
  } as const;
  const transport: LauncherInteractionTransport = {
    getState: vi.fn().mockResolvedValue(state),
    subscribe: (handler) => {
      publish = handler;
      return () => {};
    },
    keyboard: vi.fn().mockResolvedValue(undefined),
    letter: vi.fn().mockResolvedValue(undefined),
    activate: vi.fn().mockResolvedValue(undefined),
  };
  render(
    <LauncherView
      presentation={presentation}
      onActivate={() => {}}
      interactionTransport={transport}
    />,
  );
  expect(await screen.findByRole("button", { name: "Keyboard navigation" })).toBeVisible();
  expect(screen.getByRole("button", { name: "Windowless Helper" })).toHaveAttribute(
    "aria-current",
    "true",
  );
  fireEvent.click(screen.getByRole("button", { name: "Keyboard navigation" }));
  expect(transport.keyboard).toHaveBeenCalledWith(9, "display-main", 12, 4, 2, true);
  act(() => publish({ ...state, admission: 3, sequence: 1, keyboardMode: true }));
  const input = await screen.findByRole("textbox", { name: "Type a letter" });
  fireEvent.input(input, { target: { value: "n" }, inputType: "insertText" });
  expect(transport.letter).toHaveBeenCalledWith(9, "display-main", 12, 4, 3, 2, "n", 0, false);
  fireEvent.keyDown(input, { key: "Enter" });
  expect(transport.activate).not.toHaveBeenCalled();
  fireEvent.keyDown(input, { key: "v", metaKey: true });
  fireEvent.input(input, { target: { value: "p" }, inputType: "insertFromPaste" });
  fireEvent.keyUp(input, { key: "v", metaKey: true });
  fireEvent.input(input, { target: { value: "ignored" }, inputType: "insertFromDrop" });
  expect(transport.letter).toHaveBeenCalledTimes(1);
  fireEvent.compositionStart(input);
  fireEvent.keyDown(input, { key: "Enter", isComposing: true });
  expect(transport.activate).not.toHaveBeenCalled();
  fireEvent.keyUp(input, { key: "Process" });
  fireEvent.keyDown(input, { key: "Enter", isComposing: false });
  expect(transport.activate).not.toHaveBeenCalled();
  fireEvent.input(input, {
    target: { value: "に" },
    inputType: "insertCompositionText",
    isComposing: true,
  });
  expect(transport.letter).toHaveBeenCalledTimes(1);
  fireEvent.compositionEnd(input, { data: "に" });
  expect(transport.letter).toHaveBeenLastCalledWith(9, "display-main", 12, 4, 3, 3, "に", 0, false);
  fireEvent.input(input, { target: { value: "pasted" }, inputType: "insertFromPaste" });
  fireEvent.compositionStart(input);
  fireEvent.compositionEnd(input, { data: "" });
  expect(transport.letter).toHaveBeenCalledTimes(2);
  act(() =>
    publish({
      ...state,
      admission: 4,
      sequence: 1,
      keyboardMode: true,
      configured: { ...state.configured, enterActivates: true },
    }),
  );
  const activeInput = await screen.findByRole("textbox", { name: "Type a letter" });
  fireEvent.keyDown(activeInput, { key: "Enter", keyCode: 229 });
  expect(transport.activate).not.toHaveBeenCalled();
  fireEvent.keyDown(activeInput, { key: "Enter" });
  expect(transport.activate).toHaveBeenCalledWith(9, "display-main", 12, 4, 4, 2);
  fireEvent.keyDown(input, { key: "Escape" });
  expect(transport.keyboard).toHaveBeenLastCalledWith(9, "display-main", 12, 4, 4, false);
});

it("keeps decorations unfocusable and opens group members individually", () => {
  const activate = vi.fn();
  const value = {
    ...presentation,
    items: [
      { id: "space", name: "Space", icon: "", kind: "spacer" },
      { id: "line", name: "Line", icon: "", kind: "separator" },
      {
        id: "group",
        name: "Work",
        icon: "",
        kind: "group",
        status: "ready",
        members: [{ id: "member", name: "Editor", icon: "", kind: "app", status: "ready" }],
      },
      { id: "missing", name: "Moved file", icon: "", kind: "file", status: "moved" },
    ],
  };
  render(<LauncherView presentation={value} onActivate={activate} />);
  expect(screen.queryByRole("button", { name: "Space" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Line" })).toBeNull();
  expect(screen.getByRole("button", { name: "Moved file" })).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: "Work" }));
  expect(activate).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Editor" }));
  expect(activate).toHaveBeenCalledExactlyOnceWith(9, "display-main", 12, 4, "member");
});

it("offers relaunch only for an exact pinned running application", () => {
  const relaunch = vi.fn();
  render(
    <LauncherView
      presentation={{
        ...presentation,
        items: [
          {
            id: "pin",
            name: "Editor",
            icon: "",
            kind: "app",
            status: "ready",
            running: true,
            referenceRevision: 10,
          },
        ],
      }}
      onActivate={() => {}}
      onRelaunch={relaunch}
    />,
  );
  fireEvent.contextMenu(screen.getByRole("button", { name: "Editor" }));
  expect(relaunch).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Relaunch" }));
  expect(relaunch).toHaveBeenCalledExactlyOnceWith(9, "display-main", 12, 4, "pin");
});

it("opens a folder panel on primary click and app show-all from context", () => {
  const show = vi.fn();
  render(
    <LauncherView
      presentation={{
        ...presentation,
        items: [
          { id: "docs", name: "Documents", icon: "", kind: "folder", status: "ready" },
          { id: "editor", name: "Editor", icon: "", kind: "app", status: "ready", running: true },
        ],
      }}
      onActivate={() => {}}
      onShowPanel={show}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Documents" }));
  expect(show).toHaveBeenCalledWith(9, "display-main", 12, 4, "docs");
  fireEvent.contextMenu(screen.getByRole("button", { name: "Editor" }));
  fireEvent.click(screen.getByRole("button", { name: "Show all windows" }));
  expect(show).toHaveBeenCalledWith(9, "display-main", 12, 4, "editor");
});

it("keeps a group open through clock revisions and dispatches the latest scope", () => {
  const activate = vi.fn();
  const items = [
    {
      id: "group",
      name: "Work",
      icon: "",
      kind: "group",
      status: "ready",
      members: [
        {
          id: "editor",
          name: "Editor",
          icon: "",
          kind: "app",
          status: "ready",
          referenceRevision: 10,
        },
      ],
    },
  ];
  const { rerender } = render(
    <LauncherView presentation={{ ...presentation, items }} onActivate={activate} />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Work" }));
  rerender(
    <LauncherView presentation={{ ...presentation, revision: 5, items }} onActivate={activate} />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Editor" }));
  expect(activate).toHaveBeenCalledExactlyOnceWith(9, "display-main", 12, 5, "editor");
  rerender(
    <LauncherView
      presentation={{
        ...presentation,
        revision: 6,
        items: [{ ...items[0], members: [{ ...items[0].members[0], referenceRevision: 11 }] }],
      }}
      onActivate={activate}
    />,
  );
  expect(screen.queryByRole("button", { name: "Editor" })).toBeNull();
});

it("retains explicit folder root open with the latest clock revision", () => {
  const activate = vi.fn(),
    show = vi.fn();
  const p = {
    ...presentation,
    items: [{ id: "docs", name: "Documents", icon: "", kind: "folder", status: "ready" }],
  };
  const { rerender } = render(
    <LauncherView presentation={p} onActivate={activate} onShowPanel={show} />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Documents" }));
  expect(show).toHaveBeenCalledTimes(1);
  fireEvent.contextMenu(screen.getByRole("button", { name: "Documents" }));
  rerender(
    <LauncherView presentation={{ ...p, revision: 5 }} onActivate={activate} onShowPanel={show} />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Open folder" }));
  expect(activate).toHaveBeenCalledWith(9, "display-main", 12, 5, "docs");
});

it("uses the resolved backend envelope without transforming action hitboxes", () => {
  const { container } = render(
    <LauncherView
      presentation={{
        ...presentation,
        magnification: { enabled: true, scale: 2, reach: 2, primaryInset: 112, crossInset: 24 },
      }}
      onActivate={() => {}}
    />,
  );
  const strip = container.querySelector<HTMLElement>(".ot-launcher-strip")!;
  expect(strip.style.getPropertyValue("--ot-mag-primary")).toBe("108px");
  expect(strip.querySelector(".ot-launcher-visual")).not.toBeNull();
  expect(strip.querySelector<HTMLElement>("button")!.style.transform).toBe("");
});

it("offers explicit structural reorder for missing pins only with exact runtime scope", () => {
  const mutate = vi.fn();
  const p = {
    ...presentation,
    runtimeReorder: true,
    itemsRevision: "hash",
    items: [
      { id: "pin:a", name: "Missing A", icon: "", kind: "app", status: "missing" },
      { id: "pin:b", name: "B", icon: "", kind: "app" },
      { id: "app:90", name: "Running", icon: "", kind: "app" },
    ],
  };
  render(<LauncherView presentation={p} onActivate={() => {}} onMutate={mutate} />);
  expect(screen.getByRole("button", { name: "Missing A" })).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: "Reorder Missing A" }));
  fireEvent.click(screen.getByRole("button", { name: "Move later" }));
  expect(mutate).toHaveBeenCalledWith(9, "display-main", 12, 4, "hash", {
    kind: "moveAfter",
    itemID: "pin:a",
    targetID: "pin:b",
  });
  expect(screen.queryByRole("button", { name: "Reorder Running" })).toBeNull();
});

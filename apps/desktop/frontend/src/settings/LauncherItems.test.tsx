import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { makeT } from "../lib/i18n";
import type { LauncherItemStatus } from "../lib/types";
import type { LauncherItemSettingsActions } from "./LauncherItems";
import { LauncherItems } from "./LauncherItems";

function actions(): LauncherItemSettingsActions {
  return {
    load: vi
      .fn()
      .mockResolvedValue({ profileID: "work", revision: "r1", items: [], references: [] }),
    save: vi.fn(async (_p, _r, items) => ({
      profileID: "work",
      revision: "r2",
      items,
      references: [],
    })),
    chooseReference: vi.fn(async (kind) => ({
      id: `${kind.repeat(8).slice(0, 32)}`,
      kind,
      label: `Chosen ${kind}`,
      bundleID: kind === "app" ? "com.example.app" : "",
      state: "ready",
      reason: "",
      revision: 1,
    })),
    relinkReference: vi.fn(),
    cancelSelection: vi.fn(),
    chooseIcon: vi
      .fn()
      .mockResolvedValue({ id: "a".repeat(64), dataURL: "data:image/png;base64,AA==" }),
    removeReference: vi.fn(),
    removeIcon: vi.fn(),
  };
}

describe("LauncherItems", () => {
  it.each([
    ["needsSelection", "en", "Select again in Settings"],
    ["needsSelection", "pt-BR", "Selecione novamente nos Ajustes"],
    ["accessRequired", "es", "Selecciona de nuevo en Ajustes"],
    ["unknownInternalState", "en", "Item unavailable"],
  ] as const)("shows product wording for reference status %s in %s", async (state, language, expected) => {
    const a = actions();
    a.load = vi.fn().mockResolvedValue({
      profileID: "work",
      revision: "r1",
      iconIDs: [],
      items: [
        {
          id: "folder",
          kind: "folder",
          label: "Docs",
          referenceID: "f".repeat(32),
          folderView: "list",
        },
      ],
      references: [
        {
          id: "f".repeat(32),
          kind: "folder",
          label: "Docs",
          bundleID: "",
          state,
          reason: "",
          revision: 1,
        },
      ],
    });
    render(<LauncherItems profileID="work" actions={a} t={makeT(language)} />);
    expect(await screen.findByText(expected)).toBeVisible();
    expect(screen.queryByText(state)).toBeNull();
  });
  it.each([
    "availability",
    "busy completion",
    "profile persistence",
  ] as const)("recovers the saved draft after an initial rejected read on %s", async (recovery) => {
    const a = actions();
    const iconID = "d".repeat(64);
    const initial =
      recovery === "busy completion" ? { available: true, busy: true, reason: "" } : undefined;
    let publish = (_status: LauncherItemStatus) => {};
    a.status = () => new Promise(() => {});
    a.subscribe = (listener) => {
      publish = listener;
      return () => {};
    };
    const initialError =
      recovery === "profile persistence"
        ? "launcher items: profileMissing"
        : "launcher items: unavailable";
    a.load = vi
      .fn()
      .mockRejectedValueOnce(new Error(initialError))
      .mockResolvedValue({
        profileID: "work",
        revision: "saved-revision",
        references: [],
        iconIDs: [iconID],
        items: [
          { id: "guide", kind: "link", label: "Saved guide", url: "https://example.com", iconID },
        ],
      });
    a.getIcon = async (id) => ({ id, dataURL: "data:image/png;base64,AA==" });
    const t = makeT("en");
    const { rerender } = render(
      <LauncherItems profileID="work" actions={a} status={initial} refreshKey={1} t={t} />,
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(initialError);
    expect(screen.getByRole("button", { name: "Add application" })).toBeDisabled();
    if (recovery === "availability") {
      act(() => publish({ available: false, busy: false, reason: "" }));
    }
    if (recovery === "profile persistence") {
      rerender(
        <LauncherItems profileID="work" actions={a} status={initial} refreshKey={2} t={t} />,
      );
    } else {
      act(() => publish({ available: true, busy: false, reason: "" }));
    }
    await screen.findByText("Saved guide");
    expect(await screen.findByAltText("Custom icon")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByRole("button", { name: "Add application" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Save launcher items" }));
    await waitFor(() =>
      expect(a.save).toHaveBeenCalledWith("work", "saved-revision", [
        { id: "guide", kind: "link", label: "Saved guide", url: "https://example.com", iconID },
      ]),
    );
  });
  it("preserves an unsaved draft when a completed chooser refreshes metadata", async () => {
    const a = actions();
    let publish = (_status: LauncherItemStatus) => {};
    a.status = () => new Promise(() => {});
    a.subscribe = (listener) => {
      publish = listener;
      return () => {};
    };
    a.load = vi
      .fn()
      .mockResolvedValueOnce({
        profileID: "work",
        revision: "draft-base",
        iconIDs: [],
        references: [],
        items: [{ id: "guide", kind: "link", label: "Saved guide", url: "https://example.com" }],
      })
      .mockResolvedValue({
        profileID: "work",
        revision: "new-revision",
        iconIDs: [],
        items: [],
        references: [
          {
            id: "e".repeat(32),
            kind: "file",
            label: "New selection",
            bundleID: "",
            state: "ready",
            reason: "",
            revision: 1,
          },
        ],
      });
    const t = makeT("en");
    const { rerender } = render(
      <LauncherItems profileID="work" actions={a} refreshKey={1} t={t} />,
    );
    await screen.findByText("Saved guide");
    fireEvent.click(screen.getByRole("button", { name: "Add spacer" }));
    rerender(<LauncherItems profileID="work" actions={a} refreshKey={2} t={t} />);
    act(() => publish({ available: false, busy: false, reason: "" }));
    act(() => publish({ available: true, busy: false, reason: "" }));
    act(() => publish({ available: true, busy: true, reason: "" }));
    act(() => publish({ available: true, busy: false, reason: "" }));
    await screen.findByRole("button", { name: "Remove unused reference" });
    expect(screen.getAllByTestId("launcher-item")).toHaveLength(2);
    expect(screen.getByText("Saved guide")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Save launcher items" }));
    await waitFor(() =>
      expect(a.save).toHaveBeenCalledWith("work", "draft-base", [
        { id: "guide", kind: "link", label: "Saved guide", url: "https://example.com" },
        expect.objectContaining({ kind: "spacer" }),
      ]),
    );
  });
  it("ignores a late failed read after a newer profile snapshot succeeds", async () => {
    const a = actions();
    let rejectInitial = (_error: Error) => {};
    a.load = vi
      .fn()
      .mockImplementationOnce(
        () =>
          new Promise((_resolve, reject) => {
            rejectInitial = reject;
          }),
      )
      .mockResolvedValue({
        profileID: "work",
        revision: "saved",
        iconIDs: [],
        references: [],
        items: [{ id: "guide", kind: "link", label: "Saved guide", url: "https://example.com" }],
      });
    const t = makeT("en");
    const { rerender } = render(
      <LauncherItems profileID="work" actions={a} refreshKey={1} t={t} />,
    );
    rerender(<LauncherItems profileID="work" actions={a} refreshKey={2} t={t} />);
    await screen.findByText("Saved guide");
    await act(async () => rejectInitial(new Error("launcher items: profileMissing")));
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByText("Saved guide")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add application" })).toBeEnabled();
  });
  it("keeps local additions pending until the saved snapshot loads without losing typed link fields", async () => {
    const a = actions();
    let rejectInitial = (_error: Error) => {};
    a.load = vi
      .fn()
      .mockImplementationOnce(
        () =>
          new Promise((_resolve, reject) => {
            rejectInitial = reject;
          }),
      )
      .mockResolvedValue({
        profileID: "work",
        revision: "saved",
        iconIDs: [],
        references: [],
        items: [{ id: "guide", kind: "link", label: "Saved guide", url: "https://example.com" }],
      });
    const t = makeT("en");
    const { rerender } = render(
      <LauncherItems profileID="work" actions={a} refreshKey={1} t={t} />,
    );
    fireEvent.change(screen.getByLabelText("Link label"), { target: { value: "Typed guide" } });
    fireEvent.change(screen.getByLabelText("Web address"), {
      target: { value: "https://example.com/typed" },
    });
    for (const name of ["Add spacer", "Add separator", "Add link"]) {
      expect(screen.getByRole("button", { name })).toBeDisabled();
      fireEvent.click(screen.getByRole("button", { name }));
    }
    await act(async () => rejectInitial(new Error("launcher items: unavailable")));
    expect(screen.getByRole("alert")).toHaveTextContent("launcher items: unavailable");
    for (const name of ["Add spacer", "Add separator", "Add link"]) {
      expect(screen.getByRole("button", { name })).toBeDisabled();
    }
    rerender(<LauncherItems profileID="work" actions={a} refreshKey={2} t={t} />);
    await screen.findByText("Saved guide");
    expect(screen.getByLabelText("Link label")).toHaveValue("Typed guide");
    expect(screen.getByLabelText("Web address")).toHaveValue("https://example.com/typed");
    for (const name of ["Add spacer", "Add separator", "Add link"]) {
      expect(screen.getByRole("button", { name })).toBeEnabled();
      fireEvent.click(screen.getByRole("button", { name }));
    }
    expect(screen.getAllByTestId("launcher-item")).toHaveLength(4);
    expect(screen.getByText("Saved guide")).toBeInTheDocument();
    expect(screen.getByText("Typed guide")).toBeInTheDocument();
  });
  it("adds every supported item class without opening anything until an explicit chooser", async () => {
    const a = actions();
    render(<LauncherItems profileID="work" actions={a} t={makeT("en")} />);
    await screen.findByText("No launcher items yet.");
    fireEvent.click(screen.getByRole("button", { name: "Add application" }));
    await screen.findAllByText("Chosen app");
    for (const [name, count] of [
      ["Add folder", 2],
      ["Add file", 3],
      ["Add spacer", 4],
      ["Add separator", 5],
    ] as const) {
      fireEvent.click(screen.getByRole("button", { name }));
      await waitFor(() => expect(screen.getAllByTestId("launcher-item")).toHaveLength(count));
    }
    fireEvent.change(screen.getByLabelText("Link label"), { target: { value: "Guide" } });
    fireEvent.change(screen.getByLabelText("Web address"), {
      target: { value: "https://example.com/guide" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add link" }));
    await screen.findByText("Guide");
    expect(screen.getAllByTestId("launcher-item")).toHaveLength(6);
    expect(a.chooseReference).toHaveBeenCalledTimes(3);
    expect(a.save).not.toHaveBeenCalled();
  });
  it("groups only selected applications, reorders, and saves with the exact revision", async () => {
    const a = actions();
    a.load = vi.fn().mockResolvedValue({
      profileID: "work",
      revision: "exact",
      references: [],
      items: [
        { id: "app-a", kind: "app", label: "A", referenceID: "a".repeat(32) },
        { id: "app-b", kind: "app", label: "B", referenceID: "b".repeat(32) },
        { id: "gap", kind: "spacer", label: "" },
      ],
    });
    render(<LauncherItems profileID="work" actions={a} t={makeT("en")} />);
    const cards = await screen.findAllByTestId("launcher-item");
    fireEvent.click(within(cards[1]).getByRole("button", { name: "Move up" }));
    fireEvent.change(screen.getByLabelText("Group name"), { target: { value: "Work" } });
    fireEvent.click(screen.getByLabelText("A"));
    fireEvent.click(screen.getByLabelText("B"));
    fireEvent.click(screen.getByRole("button", { name: "Create group" }));
    fireEvent.click(screen.getByRole("button", { name: "Save launcher items" }));
    await waitFor(() => expect(a.save).toHaveBeenCalled());
    expect(a.save).toHaveBeenCalledWith(
      "work",
      "exact",
      expect.arrayContaining([
        expect.objectContaining({
          kind: "group",
          label: "Work",
          members: expect.arrayContaining(["app-a", "app-b"]),
        }),
      ]),
    );
  });
  it("relinks unavailable references and manages an explicit custom icon", async () => {
    const a = actions();
    a.load = vi.fn().mockResolvedValue({
      profileID: "work",
      revision: "r1",
      items: [
        {
          id: "folder",
          kind: "folder",
          label: "Docs",
          referenceID: "f".repeat(32),
          folderView: "list",
        },
      ],
      references: [
        {
          id: "f".repeat(32),
          kind: "folder",
          label: "Docs",
          bundleID: "",
          state: "missing",
          reason: "missing",
          revision: 1,
        },
      ],
    });
    a.relinkReference = vi.fn().mockResolvedValue({
      id: "f".repeat(32),
      kind: "folder",
      label: "Docs",
      bundleID: "",
      state: "ready",
      reason: "",
      revision: 2,
    });
    render(<LauncherItems profileID="work" actions={a} t={makeT("en")} />);
    await screen.findByText("Missing — relink in Settings");
    fireEvent.click(screen.getByRole("button", { name: "Relink" }));
    await waitFor(() => expect(a.relinkReference).toHaveBeenCalledWith("f".repeat(32)));
    fireEvent.click(screen.getByRole("button", { name: "Choose custom icon" }));
    await waitFor(() => expect(screen.getByAltText("Custom icon")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Remove custom icon" }));
    expect(screen.queryByAltText("Custom icon")).toBeNull();
  });
  it("offers explicit cleanup for unused chooser results and cancellation while busy", async () => {
    const a = actions();
    a.load = vi.fn().mockResolvedValue({
      profileID: "work",
      revision: "r1",
      items: [],
      references: [
        {
          id: "e".repeat(32),
          kind: "file",
          label: "Unused",
          bundleID: "",
          state: "ready",
          reason: "",
          revision: 1,
        },
      ],
    });
    render(
      <LauncherItems
        profileID="work"
        actions={a}
        t={makeT("en")}
        status={{ available: true, busy: true, reason: "" }}
      />,
    );
    const removeUnused = await screen.findByRole("button", { name: "Remove unused reference" });
    await act(async () => fireEvent.click(removeUnused));
    expect(a.removeReference).toHaveBeenCalledWith("e".repeat(32));
    fireEvent.click(screen.getByRole("button", { name: "Cancel selection" }));
    expect(a.cancelSelection).toHaveBeenCalled();
  });
  it("clears a persisted icon from the draft and only removes it after save cleanup", async () => {
    const a = actions();
    const iconID = "b".repeat(64);
    a.getIcon = vi.fn().mockResolvedValue({ id: iconID, dataURL: "data:image/png;base64,AA==" });
    a.load = vi.fn().mockResolvedValue({
      profileID: "work",
      revision: "r1",
      references: [],
      iconIDs: [iconID],
      items: [{ id: "link", kind: "link", label: "Guide", url: "https://example.com", iconID }],
    });
    render(<LauncherItems profileID="work" actions={a} t={makeT("en")} />);
    fireEvent.click(await screen.findByRole("button", { name: "Remove custom icon" }));
    expect(a.removeIcon).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Save launcher items" }));
    await waitFor(() => expect(a.save).toHaveBeenCalled());
    expect(a.save).toHaveBeenCalledWith("work", "r1", [expect.not.objectContaining({ iconID })]);
  });
  it("reorders only an internal drag token", async () => {
    const a = actions();
    a.load = vi.fn().mockResolvedValue({
      profileID: "work",
      revision: "r1",
      references: [],
      items: [
        { id: "first", kind: "link", label: "First", url: "https://example.com/1" },
        { id: "second", kind: "link", label: "Second", url: "https://example.com/2" },
      ],
    });
    render(<LauncherItems profileID="work" actions={a} t={makeT("en")} />);
    const cards = await screen.findAllByTestId("launcher-item");
    const store = new Map<string, string>();
    const dataTransfer = {
      effectAllowed: "none",
      types: ["application/x-optiontab-launcher-item"],
      setData: (type: string, value: string) => store.set(type, value),
      getData: (type: string) => store.get(type) ?? "",
    };
    fireEvent.dragStart(cards[1], { dataTransfer });
    fireEvent.dragOver(cards[0], { dataTransfer });
    fireEvent.drop(cards[0], { dataTransfer });
    fireEvent.click(screen.getByRole("button", { name: "Save launcher items" }));
    await waitFor(() => expect(a.save).toHaveBeenCalled());
    expect(vi.mocked(a.save).mock.calls[0][2].map((item) => item.id)).toEqual(["second", "first"]);
  });
  it("selects a fresh authority for an imported missing reference", async () => {
    const a = actions();
    const oldID = "c".repeat(32);
    a.load = vi.fn().mockResolvedValue({
      profileID: "work",
      revision: "r1",
      references: [],
      items: [{ id: "imported", kind: "file", label: "Old", referenceID: oldID }],
    });
    render(<LauncherItems profileID="work" actions={a} t={makeT("en")} />);
    fireEvent.click(await screen.findByRole("button", { name: "Select again" }));
    await waitFor(() => expect(a.chooseReference).toHaveBeenCalledWith("file"));
    expect(a.relinkReference).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Save launcher items" }));
    await waitFor(() => expect(a.save).toHaveBeenCalled());
    expect(vi.mocked(a.save).mock.calls[0][2][0].referenceID).not.toBe(oldID);
  });
});

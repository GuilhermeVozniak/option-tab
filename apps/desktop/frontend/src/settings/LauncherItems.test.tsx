import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { makeT } from "../lib/i18n";
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
    await screen.findByText("missing");
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
    fireEvent.click(await screen.findByRole("button", { name: "Remove unused reference" }));
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

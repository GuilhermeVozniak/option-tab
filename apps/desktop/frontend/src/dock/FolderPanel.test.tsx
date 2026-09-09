import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { DockFolderState } from "../lib/types";
import { FolderPanel } from "./FolderPanel";

const ready = (patch: Partial<DockFolderState> = {}): DockFolderState => ({
  status: "ready",
  reason: "",
  folderIdentity: "file:///tmp/Fixture",
  entries: [
    {
      id: "opaque-1",
      name: "A very long document name that must truncate.txt",
      kind: "file",
      size: 2048,
      modifiedAtMs: 1_700_000_000_000,
      hidden: false,
    },
    {
      id: "opaque-2",
      name: "Projects",
      kind: "folder",
      size: 0,
      modifiedAtMs: 1_700_000_100_000,
      hidden: false,
    },
  ],
  sort: { field: "name", direction: "asc", foldersFirst: true },
  partial: false,
  revision: 4,
  ...patch,
});

const handlers = () => ({
  onSort: vi.fn(),
  onRequestAccess: vi.fn(() => Promise.resolve()),
  onCancelAccess: vi.fn(),
  onOpen: vi.fn(() => Promise.resolve()),
});

describe("FolderPanel", () => {
  it("renders accessible exact items and opens only opaque IDs", () => {
    const h = handlers();
    render(<FolderPanel session={7} revision={9} folder={ready()} handlers={h} />);
    const document = screen.getByRole("button", { name: /A very long document/ });
    expect(document.querySelector(".ot-folder-name")).toHaveAttribute(
      "title",
      "A very long document name that must truncate.txt",
    );
    fireEvent.click(screen.getByRole("button", { name: "Open Projects" }));
    expect(h.onOpen).toHaveBeenCalledWith(7, 9, "opaque-2");
  });

  it("renders the same accessible entries as a bounded grid", () => {
    const h = handlers();
    const { container } = render(
      <FolderPanel session={7} revision={9} folder={ready()} handlers={h} view="grid" />,
    );
    expect(container.querySelector(".ot-folder-list")).toHaveClass("is-grid");
    expect(screen.getByRole("button", { name: "Open Projects" })).toBeVisible();
    expect(screen.getByText("A very long document name that must truncate.txt")).toHaveAttribute(
      "title",
      "A very long document name that must truncate.txt",
    );
    fireEvent.click(screen.getByRole("button", { name: "Open Projects" }));
    expect(h.onOpen).toHaveBeenCalledWith(7, 9, "opaque-2");
  });

  it("changes sort and folders-first with the outer presentation scope", () => {
    const h = handlers();
    render(<FolderPanel session={7} revision={9} folder={ready()} handlers={h} />);
    fireEvent.change(screen.getByLabelText("Sort folder contents by"), {
      target: { value: "size" },
    });
    expect(h.onSort).toHaveBeenCalledWith(7, 9, "size", "asc", true);
    fireEvent.click(screen.getByLabelText("Folders first"));
    expect(h.onSort).toHaveBeenCalledWith(7, 9, "name", "asc", false);
  });

  it.each([
    ["permissionRequired", "Allow access", "Folder access is required"],
    ["revoked", "Allow access", "Folder access expired"],
    ["missing", "Retry", "Folder is no longer available"],
    ["unavailable", "Retry", "Folder contents are unavailable"],
  ] as const)("renders %s guidance", async (status, action, message) => {
    const h = handlers();
    render(
      <FolderPanel session={7} revision={9} folder={ready({ status, entries: [] })} handlers={h} />,
    );
    expect(screen.getByText(message)).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: action }));
    if (status === "permissionRequired" || status === "revoked")
      await waitFor(() => expect(h.onRequestAccess).toHaveBeenCalledWith(7, 9));
    else expect(h.onSort).toHaveBeenCalledWith(7, 9, "name", "asc", true);
  });

  it("announces partial listings and action errors", async () => {
    const h = handlers();
    h.onOpen.mockRejectedValueOnce(new Error("Item changed on disk"));
    render(
      <FolderPanel
        session={7}
        revision={9}
        folder={ready({ status: "partial", partial: true })}
        handlers={h}
      />,
    );
    expect(screen.getByText("Some folder items could not be shown.")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Open Projects" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Item changed on disk");
  });

  it("cancels the exact accepted access scope after a newer update", async () => {
    let finish!: () => void;
    const h = handlers();
    h.onRequestAccess.mockReturnValueOnce(new Promise<void>((resolve) => (finish = resolve)));
    const { rerender } = render(
      <FolderPanel
        session={7}
        revision={9}
        folder={ready({ status: "permissionRequired", entries: [] })}
        handlers={h}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Allow access" }));
    expect(await screen.findByRole("button", { name: "Cancel access request" })).toBeVisible();
    rerender(
      <FolderPanel
        session={7}
        revision={10}
        folder={ready({ status: "permissionRequired", entries: [] })}
        handlers={h}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Cancel access request" }));
    expect(h.onCancelAccess).toHaveBeenCalledWith(7, 9);
    await act(async () => finish());
  });

  it("does not surface an old action failure in a newer revision", async () => {
    let reject!: (error: Error) => void;
    const h = handlers();
    h.onOpen.mockReturnValueOnce(new Promise<void>((_, fail) => (reject = fail)));
    const { rerender } = render(
      <FolderPanel session={7} revision={9} folder={ready()} handlers={h} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Open Projects" }));
    rerender(<FolderPanel session={7} revision={10} folder={ready()} handlers={h} />);
    await act(async () => reject(new Error("stale failure")));
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

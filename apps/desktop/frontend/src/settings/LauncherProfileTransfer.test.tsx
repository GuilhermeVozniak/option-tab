import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { makeT } from "../lib/i18n";
import { LauncherProfileTransfer } from "./LauncherProfileTransfer";

const review = {
  digest: "digest-1",
  revision: "dock-revision-1",
  name: "Travel",
  itemCount: 3,
  widgetCount: 2,
  notices: ["selectionsRequireRepair", "widgetsDisabled", "iconsNotIncluded"],
};

describe("LauncherProfileTransfer", () => {
  it("waits for native save completion before reporting success", async () => {
    let resolve!: (result: { status: "saved" | "cancelled" }) => void;
    const actions = {
      exportProfile: vi.fn().mockReturnValue(new Promise((done) => (resolve = done))),
      previewImport: vi.fn(),
      importProfile: vi.fn(),
    };
    render(<LauncherProfileTransfer profileID="default" t={makeT("en")} actions={actions} />);
    fireEvent.click(screen.getByRole("button", { name: "Export profile" }));
    expect(screen.getByRole("button", { name: "Exporting…" })).toBeDisabled();
    expect(screen.queryByRole("status")).toBeNull();
    resolve({ status: "saved" });
    expect(await screen.findByRole("status")).toHaveTextContent("Profile exported.");
    expect(actions.exportProfile).toHaveBeenCalledWith("default");
    expect(screen.getByRole("button", { name: "Export profile" })).toBeEnabled();
  });

  it("treats native cancellation neutrally and permits retry after errors", async () => {
    const actions = {
      exportProfile: vi
        .fn()
        .mockResolvedValueOnce({ status: "cancelled" })
        .mockRejectedValueOnce(new Error("json export: destinationExists"))
        .mockResolvedValueOnce({ status: "saved" }),
      previewImport: vi.fn(),
      importProfile: vi.fn(),
    };
    render(<LauncherProfileTransfer profileID="default" t={makeT("en")} actions={actions} />);
    fireEvent.click(screen.getByRole("button", { name: "Export profile" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Export profile" })).toBeEnabled(),
    );
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Export profile" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "That filename already exists. Choose a new name.",
    );
    fireEvent.click(screen.getByRole("button", { name: "Export profile" }));
    expect(await screen.findByRole("status")).toHaveTextContent("Profile exported.");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("reviews exact bounded bytes and commits only after confirmation", async () => {
    const actions = {
      exportProfile: vi.fn(),
      previewImport: vi.fn().mockResolvedValue(review),
      importProfile: vi.fn().mockResolvedValue({ profileID: "imported-1", settingsJSON: "{}" }),
    };
    const onImported = vi.fn();
    const { container } = render(
      <LauncherProfileTransfer
        profileID="default"
        t={makeT("en")}
        actions={actions}
        onImported={onImported}
      />,
    );
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    const document = JSON.stringify({ format: "option-tab.launcher-profile", version: 1 });
    fireEvent.change(input, { target: { files: [new File([document], "travel.json")] } });
    expect(await screen.findByText("Travel")).toBeVisible();
    expect(screen.getByText("3 items · 2 widgets")).toBeVisible();
    expect(actions.importProfile).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Import reviewed profile" }));
    await waitFor(() =>
      expect(actions.importProfile).toHaveBeenCalledWith(document, "digest-1", "dock-revision-1"),
    );
    expect(onImported).toHaveBeenCalledWith("imported-1");
  });

  it("cancels locally and ignores a stale preview completion", async () => {
    let resolve!: (value: typeof review) => void;
    const actions = {
      exportProfile: vi.fn(),
      previewImport: vi.fn().mockReturnValue(new Promise((done) => (resolve = done))),
      importProfile: vi.fn(),
    };
    const { container, rerender } = render(
      <LauncherProfileTransfer profileID="first" t={makeT("en")} actions={actions} />,
    );
    fireEvent.change(container.querySelector('input[type="file"]')!, {
      target: { files: [new File(["{}"], "first.json")] },
    });
    rerender(<LauncherProfileTransfer profileID="second" t={makeT("en")} actions={actions} />);
    resolve(review);
    await Promise.resolve();
    expect(screen.queryByText("Travel")).toBeNull();
  });

  it("rejects oversized files before calling Go", async () => {
    const actions = {
      exportProfile: vi.fn(),
      previewImport: vi.fn(),
      importProfile: vi.fn(),
    };
    const { container } = render(
      <LauncherProfileTransfer profileID="default" t={makeT("en")} actions={actions} />,
    );
    fireEvent.change(container.querySelector('input[type="file"]')!, {
      target: { files: [new File([new Uint8Array(256 * 1024 + 1)], "large.json")] },
    });
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Profile file is larger than 256 KiB.",
    );
    expect(actions.previewImport).not.toHaveBeenCalled();
  });

  it("ignores a native save completion belonging to the previous profile", async () => {
    let resolve!: (result: { status: "saved" }) => void;
    const actions = {
      exportProfile: vi.fn().mockReturnValue(new Promise((done) => (resolve = done))),
      previewImport: vi.fn(),
      importProfile: vi.fn(),
    };
    const view = render(
      <LauncherProfileTransfer profileID="first" t={makeT("en")} actions={actions} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Export profile" }));
    view.rerender(<LauncherProfileTransfer profileID="second" t={makeT("en")} actions={actions} />);
    resolve({ status: "saved" });
    await Promise.resolve();
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.getByRole("button", { name: "Export profile" })).toBeEnabled();
  });

  it("does not send a document whose file read completes after unmount", async () => {
    let resolve!: (document: string) => void;
    const actions = {
      exportProfile: vi.fn(),
      previewImport: vi.fn(),
      importProfile: vi.fn(),
    };
    const file = {
      size: 2,
      text: () => new Promise<string>((done) => (resolve = done)),
    } as File;
    const view = render(
      <LauncherProfileTransfer profileID="default" t={makeT("en")} actions={actions} />,
    );
    fireEvent.change(screen.getByLabelText("Import profile file"), {
      target: { files: [file] },
    });
    view.unmount();
    resolve("{}");
    await Promise.resolve();
    expect(actions.previewImport).not.toHaveBeenCalled();
  });
});

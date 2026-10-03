import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { saveSettingsExport } from "../lib/json-export-bridge";
import { defaultSettings } from "../lib/types";
import { Settings } from "./Settings";

const native = vi.hoisted(() => ({ save: vi.fn() }));
vi.mock("../../bindings/option-tab/app.js", () => ({ SaveSettingsExport: native.save }));

beforeEach(() => {
  native.save.mockReset();
});

describe("native settings export", () => {
  it("clears an export collision after a successful settings import", async () => {
    native.save.mockRejectedValue(new Error("json export: destinationExists"));
    const onImport = vi.fn().mockResolvedValue(undefined);
    const { container } = render(
      <Settings
        settings={defaultSettings}
        onChange={vi.fn()}
        onExport={saveSettingsExport}
        onImport={onImport}
      />,
    );
    fireEvent.click(screen.getByLabelText("Export settings"));
    expect(await screen.findByRole("alert")).toHaveTextContent("That filename already exists.");
    const file = new File(["{}"], "settings.json");
    Object.defineProperty(file, "text", { value: () => Promise.resolve("{}") });
    fireEvent.change(container.querySelector('input[type="file"]')!, { target: { files: [file] } });
    await waitFor(() => expect(onImport).toHaveBeenCalledWith("{}"));
    expect(await screen.findByText("Settings imported.")).toBeVisible();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("keeps a cancelled import neutral and reports success only after persistence", async () => {
    let finish!: () => void;
    const onImport = vi.fn().mockReturnValue(
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
    );
    const { container } = render(
      <Settings settings={defaultSettings} onChange={vi.fn()} onImport={onImport} />,
    );
    const input = container.querySelector('input[type="file"]')!;
    fireEvent.change(input, { target: { files: [] } });
    expect(onImport).not.toHaveBeenCalled();
    expect(screen.queryByText("Settings imported.")).toBeNull();
    const file = new File(["{}"], "settings.json");
    Object.defineProperty(file, "text", { value: () => Promise.resolve("{}") });
    fireEvent.change(input, { target: { files: [file] } });
    await waitFor(() => expect(onImport).toHaveBeenCalledTimes(1));
    expect(screen.queryByText("Settings imported.")).toBeNull();
    expect(screen.getByLabelText("Import settings")).toBeDisabled();
    finish();
    expect(await screen.findByText("Settings imported.")).toBeVisible();
    fireEvent.change(input, { target: { files: [] } });
    expect(screen.getByText("Settings imported.")).toBeVisible();
  });

  it("exports through the native chooser and reports only a completed save", async () => {
    let resolve!: (result: { status: string }) => void;
    native.save.mockReturnValue(new Promise((done) => (resolve = done)));
    const onChange = vi.fn();
    render(
      <Settings settings={defaultSettings} onChange={onChange} onExport={saveSettingsExport} />,
    );
    fireEvent.click(screen.getByLabelText("Export settings"));
    expect(native.save).toHaveBeenCalledWith();
    expect(screen.getByLabelText("Export settings")).toBeDisabled();
    expect(screen.queryByText("Settings exported.")).toBeNull();
    resolve({ status: "saved" });
    expect(await screen.findByText("Settings exported.")).toBeVisible();
    expect(screen.getByLabelText("Export settings")).toBeEnabled();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("keeps cancellation neutral and shows a retryable coarse failure", async () => {
    native.save
      .mockResolvedValueOnce({ status: "cancelled" })
      .mockRejectedValueOnce(new Error("json export: ioFailure"));
    render(
      <Settings settings={defaultSettings} onChange={vi.fn()} onExport={saveSettingsExport} />,
    );
    fireEvent.click(screen.getByLabelText("Export settings"));
    await waitFor(() => expect(native.save).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.getByLabelText("Export settings")).toBeEnabled());
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText("Settings exported.")).toBeNull();
    fireEvent.click(screen.getByLabelText("Export settings"));
    expect(await screen.findByRole("alert")).toHaveTextContent("The file could not be exported.");
    expect(screen.getByLabelText("Export settings")).toBeEnabled();
  });
});

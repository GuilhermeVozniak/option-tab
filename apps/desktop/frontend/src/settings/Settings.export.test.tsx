import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { saveSettingsExport } from "../lib/json-export-bridge";
import { defaultSettings } from "../lib/types";
import { Settings } from "./Settings";

const native = vi.hoisted(() => ({ save: vi.fn() }));
vi.mock("../../bindings/option-tab/app.js", () => ({ SaveSettingsExport: native.save }));

beforeEach(() => native.save.mockReset());

describe("native settings export", () => {
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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { makeT } from "../../lib/i18n";
import { defaultSettings } from "../../lib/types";
import type { TabContext } from "../shared";
import { DockTab } from "./DockTab";

function context(): TabContext {
  return {
    settings: structuredClone(defaultSettings),
    t: makeT("en"),
    onChange: vi.fn(),
    patch: vi.fn(),
    patchAppearance: vi.fn(),
    patchBehavior: vi.fn(),
    patchFilters: vi.fn(),
    patchShortcut: vi.fn(),
    mode: "windows",
    modeAppearance: defaultSettings.appearance,
    modeBehavior: defaultSettings.behavior,
    modePlacement: defaultSettings.placement,
    patchModeAppearance: vi.fn(),
    patchModeBehavior: vi.fn(),
    patchModePreferences: vi.fn(),
  };
}

it("shows one Dock area at a time and preserves the launcher draft while navigating", () => {
  const ctx = context();
  render(<DockTab ctx={ctx} />);
  const navigation = within(screen.getByRole("navigation", { name: "Dock sections" }));
  const name = screen.getByRole("textbox", { name: "Profile name" });
  fireEvent.change(name, { target: { value: "Writing draft" } });

  expect(navigation.getByRole("button", { name: "Launcher" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  expect(screen.queryByRole("switch", { name: "Enable Dock previews" })).toBeNull();
  fireEvent.click(navigation.getByRole("button", { name: "Window previews" }));
  expect(screen.getByRole("switch", { name: "Enable Dock previews" })).toBeVisible();
  expect(name).toBeInTheDocument();
  expect(name).not.toBeVisible();
  expect(screen.queryByRole("switch", { name: "Enable media controls" })).toBeNull();

  fireEvent.click(navigation.getByRole("button", { name: "Media" }));
  expect(screen.getByRole("switch", { name: "Enable media controls" })).toBeVisible();
  fireEvent.click(navigation.getByRole("button", { name: "Monitor" }));
  expect(screen.getByRole("heading", { name: "Dock monitor lock" })).toBeVisible();
  fireEvent.click(navigation.getByRole("button", { name: "Launcher" }));
  expect(screen.getByRole("textbox", { name: "Profile name" })).toBe(name);
  expect(name).toHaveValue("Writing draft");
  expect(ctx.patch).not.toHaveBeenCalled();
});

it("keeps preview permissions explicit and media connection gates intact after navigation", () => {
  const ctx = context();
  ctx.settings.dock.media = {
    enabled: false,
    musicEnabled: true,
    spotifyEnabled: false,
    remoteArtwork: false,
  };
  const onRequest = vi.fn();
  const onConnect = vi.fn();
  render(
    <DockTab
      ctx={ctx}
      permissions={{
        state: { accessibility: "denied", screenRecording: "denied" },
        onRequest,
        onOpenSettings: vi.fn(),
      }}
      media={{
        permissions: { music: { status: "notConnected", reason: "Not connected" } },
        onConnect,
      }}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Media" }));
  expect(screen.getByRole("button", { name: "Connect" })).toBeDisabled();
  expect(onConnect).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Window previews" }));
  expect(onRequest).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("switch", { name: "Enable Dock previews" }));
  expect(ctx.patch).toHaveBeenCalledWith(
    expect.objectContaining({ dock: expect.objectContaining({ enabled: true }) }),
  );
  expect(onRequest.mock.calls).toEqual([["accessibility"], ["screenRecording"]]);
});

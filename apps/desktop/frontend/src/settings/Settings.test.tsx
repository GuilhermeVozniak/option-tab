import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { defaultSettings, type Shortcut } from "../lib/types";
import { PROJECT_URL, Settings } from "./Settings";

// makeShortcuts builds n minimal shortcuts with ids 1..n for boundary tests.
const makeShortcuts = (n: number): Shortcut[] =>
  Array.from({ length: n }, (_, i) => ({
    id: i + 1,
    chord: "",
    enabled: true,
    scope: { appScope: "all" as const },
  }));

describe("Settings", () => {
  it("renders current values", () => {
    render(<Settings settings={defaultSettings} onChange={vi.fn()} />);
    expect(screen.getByLabelText("Visual style thumbnails")).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect((screen.getByLabelText("Hold modifier to cycle") as HTMLInputElement).checked).toBe(
      true,
    );
  });

  it("emits changes for the visual style", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("Visual style titles"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ appearance: expect.objectContaining({ style: "titles" }) }),
    );
  });

  it("applies a size preset to the pixel knobs", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("Size large"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        appearance: expect.objectContaining({
          sizePreset: "large",
          thumbnailMaxPx: 360,
          iconSizePx: 96,
        }),
      }),
    );
  });

  it("toggles behavior flags", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("Start at login"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ behavior: expect.objectContaining({ startAtLogin: true }) }),
    );
  });

  it("records a shortcut chord from the keys pressed", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    const chord = screen.getByLabelText("Shortcut 1 chord");
    fireEvent.keyDown(chord, { code: "Tab", ctrlKey: true });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        shortcuts: expect.arrayContaining([
          expect.objectContaining({ id: 1, chord: "control+tab" }),
        ]),
      }),
    );
  });

  it("changes the display order", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Display order"), { target: { value: "alphabetical" } });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ order: "alphabetical" }));
  });

  it("switches tabs and exposes filtering + blacklist controls", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    // Tabs are present.
    expect(screen.getByRole("tab", { name: "Filtering" })).toBeInTheDocument();
    // Filtering controls exist (panels stay mounted) and emit changes.
    fireEvent.change(screen.getByLabelText("Spaces"), { target: { value: "active" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ filters: expect.objectContaining({ spaces: "active" }) }),
    );
    fireEvent.change(screen.getByLabelText("Show fullscreen windows"), {
      target: { value: "hide" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ filters: expect.objectContaining({ showFullscreen: "hide" }) }),
    );
    fireEvent.change(screen.getByLabelText("Show minimized windows"), {
      target: { value: "showAtEnd" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ filters: expect.objectContaining({ showMinimized: "showAtEnd" }) }),
    );
  });

  it("adds and removes shortcuts within the 1..9 range", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByText("+ Add shortcut"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        shortcuts: expect.arrayContaining([expect.objectContaining({ id: 3 })]),
      }),
    );
    fireEvent.click(screen.getByLabelText("Remove shortcut 2"));
    const removed = onChange.mock.calls.at(-1)?.[0];
    expect(removed.shortcuts.some((s: { id: number }) => s.id === 2)).toBe(false);
  });

  it("adds a structured blacklist entry", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByText("+ Add app"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        filters: expect.objectContaining({
          appBlacklist: [{ match: "", hide: "always", ignoreShortcuts: false }],
        }),
      }),
    );
  });

  it("edits blacklist hide mode and ignore-shortcuts", () => {
    const onChange = vi.fn();
    const custom = {
      ...defaultSettings,
      filters: {
        ...defaultSettings.filters,
        appBlacklist: [{ match: "com.game", hide: "always" as const, ignoreShortcuts: false }],
      },
    };
    render(<Settings settings={custom} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Blacklist hide 1"), {
      target: { value: "whenNoWindow" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        filters: expect.objectContaining({
          appBlacklist: [expect.objectContaining({ hide: "whenNoWindow" })],
        }),
      }),
    );
    fireEvent.click(screen.getByLabelText("Blacklist ignore shortcuts 1"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        filters: expect.objectContaining({
          appBlacklist: [expect.objectContaining({ ignoreShortcuts: true })],
        }),
      }),
    );
  });

  it("selects the menubar icon style, including hidden", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("Menubar icon outline"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        behavior: expect.objectContaining({ showMenubarIcon: true, menubarIconStyle: "outline" }),
      }),
    );
    fireEvent.click(screen.getByLabelText("Menubar icon hidden"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ behavior: expect.objectContaining({ showMenubarIcon: false }) }),
    );
  });

  it("sets update and crash-report policies", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("Updates off"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ behavior: expect.objectContaining({ updatePolicy: "off" }) }),
    );
    fireEvent.click(screen.getByLabelText("Crash reports never"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ behavior: expect.objectContaining({ crashReports: "never" }) }),
    );
  });

  it("configures per-shortcut release action and order override", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Shortcut 1 when released"), {
      target: { value: "doNothing" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        shortcuts: expect.arrayContaining([
          expect.objectContaining({ id: 1, whenReleased: "doNothing" }),
        ]),
      }),
    );
    fireEvent.change(screen.getByLabelText("Shortcut 1 order"), {
      target: { value: "recentlyCreated" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        shortcuts: expect.arrayContaining([
          expect.objectContaining({
            id: 1,
            scope: expect.objectContaining({ order: "recentlyCreated" }),
          }),
        ]),
      }),
    );
  });

  it("toggles mouse-hover selection and cursor-follow-focus", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("Select windows on mouse hover"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ behavior: expect.objectContaining({ mouseHoverSelect: false }) }),
    );
    fireEvent.click(screen.getByLabelText("Cursor follows focus"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ behavior: expect.objectContaining({ cursorFollowFocus: true }) }),
    );
  });

  it("exposes appearance parity controls", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Window title truncation"), {
      target: { value: "middle" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        appearance: expect.objectContaining({ titleTruncation: "middle" }),
      }),
    );
    fireEvent.change(screen.getByLabelText("Apparition delay"), { target: { value: "500" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        appearance: expect.objectContaining({ apparitionDelayMs: 500 }),
      }),
    );
    fireEvent.click(screen.getByLabelText("Preview selected window"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ appearance: expect.objectContaining({ previewSelected: true }) }),
    );
  });

  it("renders translated copy when a language is selected", () => {
    const custom = {
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, language: "pt-BR" },
    };
    render(<Settings settings={custom} onChange={vi.fn()} />);
    expect(screen.getByText("Iniciar no login")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Aparência" })).toBeInTheDocument();
    // Aria-labels stay English (stable identifiers).
    expect(screen.getByLabelText("Start at login")).toBeInTheDocument();
  });

  it("shows the update banner when a newer release is available", () => {
    const onInstallUpdate = vi.fn();
    render(
      <Settings
        settings={defaultSettings}
        onChange={vi.fn()}
        about={{
          version: "0.1.0",
          update: { version: "v0.2.0", url: "https://example.com/rel" },
          onOpenURL: vi.fn(),
          onCheckUpdates: vi.fn(),
          onInstallUpdate,
        }}
      />,
    );
    expect(screen.getAllByText("Version v0.2.0 is available.").length).toBeGreaterThan(0);
    fireEvent.click(screen.getAllByLabelText("Install update")[0]);
    expect(onInstallUpdate).toHaveBeenCalled();
  });

  it("shows the self-update progress and disables the install button while running", () => {
    render(
      <Settings
        settings={defaultSettings}
        onChange={vi.fn()}
        about={{
          version: "0.1.0",
          update: { version: "v0.2.0", url: "https://example.com/rel" },
          progress: { stage: "downloading" },
          onOpenURL: vi.fn(),
          onCheckUpdates: vi.fn(),
          onInstallUpdate: vi.fn(),
        }}
      />,
    );
    expect(screen.getAllByText("Downloading update…").length).toBeGreaterThan(0);
    expect(screen.getAllByLabelText("Install update")[0]).toBeDisabled();
  });

  it("surfaces a self-update failure in the banner and re-enables the button", () => {
    render(
      <Settings
        settings={defaultSettings}
        onChange={vi.fn()}
        about={{
          version: "0.1.0",
          update: { version: "v0.2.0", url: "https://example.com/rel" },
          progress: { stage: "error", message: "mount dmg: boom" },
          onOpenURL: vi.fn(),
          onCheckUpdates: vi.fn(),
          onInstallUpdate: vi.fn(),
        }}
      />,
    );
    expect(screen.getAllByText(/Update failed\./).length).toBeGreaterThan(0);
    expect(screen.getAllByLabelText("Install update")[0]).toBeEnabled();
  });

  it("shows the crash banner with report and dismiss actions", () => {
    const onReport = vi.fn();
    const onDismiss = vi.fn();
    render(
      <Settings
        settings={defaultSettings}
        onChange={vi.fn()}
        crash={{ summary: "panic: boom", onReport, onDismiss }}
      />,
    );
    expect(screen.getByText("A crash from the previous session was detected.")).toBeInTheDocument();
    expect(screen.getByText("panic: boom")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Report crash"));
    expect(onReport).toHaveBeenCalled();
    fireEvent.click(screen.getByLabelText("Dismiss crash report"));
    expect(onDismiss).toHaveBeenCalled();
  });

  it("omits the crash banner when there is nothing to report", () => {
    render(<Settings settings={defaultSettings} onChange={vi.fn()} />);
    expect(screen.queryByText("A crash from the previous session was detected.")).toBeNull();
  });

  it("renders the About tab with version and link actions", () => {
    const onOpenURL = vi.fn();
    const onCheckUpdates = vi.fn();
    render(
      <Settings
        settings={defaultSettings}
        onChange={vi.fn()}
        about={{ version: "1.2.3", onOpenURL, onCheckUpdates }}
      />,
    );
    expect(screen.getByText("Version 1.2.3")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Open project website"));
    expect(onOpenURL).toHaveBeenCalled();
    fireEvent.click(screen.getByLabelText("Send feedback"));
    expect(onOpenURL).toHaveBeenCalledTimes(2);
    fireEvent.click(screen.getByLabelText("Check for updates"));
    expect(onCheckUpdates).toHaveBeenCalled();
  });

  it("resets to defaults", () => {
    const onChange = vi.fn();
    const custom = { ...defaultSettings, order: "alphabetical" as const };
    render(<Settings settings={custom} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("Reset to defaults"));
    expect(onChange).toHaveBeenCalledWith(defaultSettings);
  });

  it("omits the Permissions section when no permissions prop is given", () => {
    render(<Settings settings={defaultSettings} onChange={vi.fn()} />);
    expect(screen.queryByText("Permissions")).toBeNull();
  });

  it("renders permission states and triggers actions for a denied permission", () => {
    const onRequest = vi.fn();
    const onOpenSettings = vi.fn();
    // onboarded: true, otherwise the first-run wizard replaces the form.
    const onboarded = {
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    };
    render(
      <Settings
        settings={onboarded}
        onChange={vi.fn()}
        permissions={{
          state: { accessibility: "granted", screenRecording: "denied" },
          onRequest,
          onOpenSettings,
        }}
      />,
    );
    // A granted permission offers no action buttons.
    expect(screen.queryByLabelText("Grant Accessibility")).toBeNull();
    // A denied permission offers Grant + Open Settings, wired to the callbacks.
    fireEvent.click(screen.getByLabelText("Grant Screen Recording"));
    expect(onRequest).toHaveBeenCalledWith("screenRecording");
    fireEvent.click(screen.getByLabelText("Open Screen Recording settings"));
    expect(onOpenSettings).toHaveBeenCalledWith("screenRecording");
  });

  it("shows the onboarding wizard on first run and finishes it", () => {
    const onChange = vi.fn();
    render(
      <Settings
        settings={defaultSettings}
        onChange={onChange}
        permissions={{
          state: { accessibility: "unknown", screenRecording: "unknown" },
          onRequest: vi.fn(),
          onOpenSettings: vi.fn(),
        }}
      />,
    );
    // The wizard replaces the settings form until onboarding completes.
    expect(screen.getByText("Welcome to Option Tab")).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "General" })).toBeNull();
    fireEvent.click(screen.getByLabelText("Finish onboarding"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ behavior: expect.objectContaining({ onboarded: true }) }),
    );
  });

  it("skips onboarding when there are no live permissions (browser/tests)", () => {
    render(<Settings settings={defaultSettings} onChange={vi.fn()} />);
    expect(screen.queryByText("Welcome to Option Tab")).toBeNull();
    expect(screen.getByRole("tab", { name: "General" })).toBeInTheDocument();
  });

  it("disables add at 9 shortcuts and remove at 1 shortcut", () => {
    const nine = { ...defaultSettings, shortcuts: makeShortcuts(9) };
    const { unmount } = render(<Settings settings={nine} onChange={vi.fn()} />);
    expect(screen.getByText("+ Add shortcut").closest("button")).toBeDisabled();
    expect(screen.getByLabelText("Remove shortcut 9")).not.toBeDisabled();
    unmount();

    const one = { ...defaultSettings, shortcuts: makeShortcuts(1) };
    render(<Settings settings={one} onChange={vi.fn()} />);
    expect(screen.getByLabelText("Remove shortcut 1")).toBeDisabled();
    expect(screen.getByText("+ Add shortcut").closest("button")).not.toBeDisabled();
  });

  it("toggles a shortcut's enabled flag and sets/clears its style override", () => {
    const onChange = vi.fn();
    const { unmount } = render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("Shortcut 1 enabled"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        shortcuts: expect.arrayContaining([expect.objectContaining({ id: 1, enabled: false })]),
      }),
    );
    fireEvent.change(screen.getByLabelText("Shortcut 1 style"), { target: { value: "titles" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        shortcuts: expect.arrayContaining([
          expect.objectContaining({ id: 1, styleOverride: "titles" }),
        ]),
      }),
    );
    unmount();

    // Clearing: start from an explicit override and pick "Default style" ('').
    const overridden = {
      ...defaultSettings,
      shortcuts: [
        { ...defaultSettings.shortcuts[0], styleOverride: "titles" as const },
        defaultSettings.shortcuts[1],
      ],
    };
    const onClear = vi.fn();
    render(<Settings settings={overridden} onChange={onClear} />);
    fireEvent.change(screen.getByLabelText("Shortcut 1 style"), { target: { value: "" } });
    const next = onClear.mock.calls.at(-1)?.[0];
    expect(next.shortcuts[0].styleOverride).toBeUndefined();
  });

  it("sets and clears per-shortcut spaces/screens scope overrides", () => {
    const onChange = vi.fn();
    const { unmount } = render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Shortcut 1 spaces"), { target: { value: "all" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        shortcuts: expect.arrayContaining([
          expect.objectContaining({ id: 1, scope: expect.objectContaining({ spaces: "all" }) }),
        ]),
      }),
    );
    fireEvent.change(screen.getByLabelText("Shortcut 1 screens"), { target: { value: "cursor" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        shortcuts: expect.arrayContaining([
          expect.objectContaining({ id: 1, scope: expect.objectContaining({ screens: "cursor" }) }),
        ]),
      }),
    );
    unmount();

    // Clearing: overrides go back to '' -> undefined (inherit global).
    const overridden = {
      ...defaultSettings,
      shortcuts: [
        {
          ...defaultSettings.shortcuts[0],
          scope: { appScope: "all" as const, spaces: "all" as const, screens: "cursor" as const },
        },
        defaultSettings.shortcuts[1],
      ],
    };
    const onClear = vi.fn();
    render(<Settings settings={overridden} onChange={onClear} />);
    fireEvent.change(screen.getByLabelText("Shortcut 1 spaces"), { target: { value: "" } });
    expect(onClear.mock.calls.at(-1)?.[0].shortcuts[0].scope.spaces).toBeUndefined();
    fireEvent.change(screen.getByLabelText("Shortcut 1 screens"), { target: { value: "" } });
    expect(onClear.mock.calls.at(-1)?.[0].shortcuts[0].scope.screens).toBeUndefined();
  });

  it("selects the dark theme from the segmented control", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("Theme dark"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ appearance: expect.objectContaining({ theme: "dark" }) }),
    );
  });

  it("edits appearance knobs: max columns, opacity, accent color, blur", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Max columns"), { target: { value: "7" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ appearance: expect.objectContaining({ maxColumns: 7 }) }),
    );
    fireEvent.change(screen.getByLabelText("Background opacity"), { target: { value: "0.5" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ appearance: expect.objectContaining({ backgroundOpacity: 0.5 }) }),
    );
    fireEvent.change(screen.getByLabelText("Accent color"), { target: { value: "#112233" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ appearance: expect.objectContaining({ accentColor: "#112233" }) }),
    );
    // blur defaults to true; the checkbox flips it (settings emission only —
    // the overlay never reads it, so there is no overlay DOM to assert).
    fireEvent.click(screen.getByLabelText("Background blur"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ appearance: expect.objectContaining({ blur: false }) }),
    );
  });

  it("changes the overlay placement", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Overlay placement"), {
      target: { value: "activeScreen" },
    });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ placement: "activeScreen" }));
  });

  it("changes a shortcut's app scope", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    // Shortcut 1 defaults to "all"; flip to the other value.
    fireEvent.change(screen.getByLabelText("Shortcut 1 scope"), {
      target: { value: "activeApp" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        shortcuts: expect.arrayContaining([
          expect.objectContaining({
            id: 1,
            scope: expect.objectContaining({ appScope: "activeApp" }),
          }),
        ]),
      }),
    );
  });

  it("changes the global screens filter", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Screens"), { target: { value: "cursor" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ filters: expect.objectContaining({ screens: "cursor" }) }),
    );
  });

  it("changes the hidden-apps visibility", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Show hidden windows"), { target: { value: "hide" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ filters: expect.objectContaining({ showHiddenApps: "hide" }) }),
    );
  });

  it("toggles showing windows without a title", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("Show windows without a title"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        filters: expect.objectContaining({ showWindowsWithoutTitle: true }),
      }),
    );
  });

  it("edits and removes a blacklist entry, and hints when the list is empty", () => {
    const onChange = vi.fn();
    const custom = {
      ...defaultSettings,
      filters: {
        ...defaultSettings.filters,
        appBlacklist: [{ match: "old", hide: "always" as const, ignoreShortcuts: false }],
      },
    };
    const { unmount } = render(<Settings settings={custom} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Blacklist entry 1"), {
      target: { value: "com.foo" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        filters: expect.objectContaining({
          appBlacklist: [expect.objectContaining({ match: "com.foo" })],
        }),
      }),
    );
    fireEvent.click(screen.getByLabelText("Remove blacklist entry 1"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ filters: expect.objectContaining({ appBlacklist: [] }) }),
    );
    unmount();

    render(<Settings settings={defaultSettings} onChange={vi.fn()} />);
    expect(screen.getByText("No apps blacklisted.")).toBeInTheDocument();
  });

  it("changes the language", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Language"), { target: { value: "pt-BR" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ behavior: expect.objectContaining({ language: "pt-BR" }) }),
    );
  });

  it("uses dev fallbacks without the about prop and supports the project with it", () => {
    const open = vi.spyOn(window, "open").mockImplementation(() => null);
    try {
      const { unmount } = render(<Settings settings={defaultSettings} onChange={vi.fn()} />);
      expect(screen.getByText("Version dev")).toBeInTheDocument();
      fireEvent.click(screen.getByLabelText("Check for updates now"));
      expect(open).toHaveBeenCalledWith(`${PROJECT_URL}/releases`, "_blank", "noopener");
      unmount();

      const onOpenURL = vi.fn();
      render(
        <Settings
          settings={defaultSettings}
          onChange={vi.fn()}
          about={{ version: "1.0.0", onOpenURL, onCheckUpdates: vi.fn() }}
        />,
      );
      fireEvent.click(screen.getByLabelText("Support this project"));
      expect(onOpenURL).toHaveBeenCalledWith(PROJECT_URL);
    } finally {
      open.mockRestore();
    }
  });

  it("imports settings from a JSON file and ignores malformed files", async () => {
    const onChange = vi.fn();
    const { container } = render(<Settings settings={defaultSettings} onChange={onChange} />);
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    expect(input).not.toBeNull();

    const imported = { ...defaultSettings, order: "alphabetical" as const };
    const good = new File([JSON.stringify(imported)], "settings.json", {
      type: "application/json",
    });
    // jsdom's File may lack .text(); provide it on the instance.
    Object.defineProperty(good, "text", {
      value: () => Promise.resolve(JSON.stringify(imported)),
    });
    fireEvent.change(input, { target: { files: [good] } });
    await waitFor(() => expect(onChange).toHaveBeenCalledWith(imported));

    onChange.mockClear();
    const bad = new File(["{not json"], "settings.json", { type: "application/json" });
    Object.defineProperty(bad, "text", { value: () => Promise.resolve("{not json") });
    fireEvent.change(input, { target: { files: [bad] } });
    await new Promise((r) => setTimeout(r, 0));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("exports settings as a JSON download", () => {
    const createObjectURL = vi.fn((_blob: Blob) => "blob:x");
    const revokeObjectURL = vi.fn();
    const hadCreate = "createObjectURL" in URL;
    // jsdom has no createObjectURL; install stubs and clean up after.
    (URL as unknown as Record<string, unknown>).createObjectURL = createObjectURL;
    (URL as unknown as Record<string, unknown>).revokeObjectURL = revokeObjectURL;
    let download = "";
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (
      this: HTMLAnchorElement,
    ) {
      download = this.download;
    });
    try {
      render(<Settings settings={defaultSettings} onChange={vi.fn()} />);
      fireEvent.click(screen.getByLabelText("Export settings"));
      expect(createObjectURL).toHaveBeenCalledTimes(1);
      expect(createObjectURL.mock.calls[0][0]).toBeInstanceOf(Blob);
      expect(click).toHaveBeenCalledTimes(1);
      expect(download).toBe("option-tab-settings.json");
      expect(revokeObjectURL).toHaveBeenCalledWith("blob:x");
    } finally {
      click.mockRestore();
      if (!hadCreate) {
        delete (URL as unknown as Record<string, unknown>).createObjectURL;
        delete (URL as unknown as Record<string, unknown>).revokeObjectURL;
      }
    }
  });

  it("labels the onboarding finish button 'Get started' when all permissions are granted", () => {
    const onChange = vi.fn();
    render(
      <Settings
        settings={defaultSettings}
        onChange={onChange}
        permissions={{
          state: { accessibility: "granted", screenRecording: "granted" },
          onRequest: vi.fn(),
          onOpenSettings: vi.fn(),
        }}
      />,
    );
    const finish = screen.getByLabelText("Finish onboarding");
    expect(finish).toHaveTextContent("Get started");
    expect(screen.queryByText("Skip for now")).toBeNull();
    fireEvent.click(finish);
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ behavior: expect.objectContaining({ onboarded: true }) }),
    );
  });

  it("deep-links to a requested tab and navigates via the tab list", () => {
    const { container, unmount } = render(
      <Settings settings={defaultSettings} onChange={vi.fn()} requestedTab="About" />,
    );
    expect(screen.getByRole("tab", { name: "About" })).toHaveAttribute("aria-selected", "true");
    const aboutSection = container.querySelector('section[aria-label="About"]') as HTMLElement;
    expect(aboutSection.hidden).toBe(false);
    unmount();

    // An unknown requestedTab keeps the default General tab.
    const second = render(
      <Settings settings={defaultSettings} onChange={vi.fn()} requestedTab="NotATab" />,
    );
    expect(screen.getByRole("tab", { name: "General" })).toHaveAttribute("aria-selected", "true");
    fireEvent.click(screen.getByRole("tab", { name: "Appearance" }));
    expect(screen.getByRole("tab", { name: "Appearance" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByRole("tab", { name: "General" })).toHaveAttribute("aria-selected", "false");
    const generalSection = second.container.querySelector(
      'section[aria-label="General"]',
    ) as HTMLElement;
    expect(generalSection.hidden).toBe(true);
  });

  it("toggles background thumbnail capture", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("Capture windows in the background"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        behavior: expect.objectContaining({ captureInBackground: true }),
      }),
    );
  });
});

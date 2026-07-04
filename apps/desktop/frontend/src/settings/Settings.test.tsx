import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { defaultSettings } from "../lib/types";
import { Settings } from "./Settings";

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
    const onOpenURL = vi.fn();
    render(
      <Settings
        settings={defaultSettings}
        onChange={vi.fn()}
        about={{
          version: "0.1.0",
          update: { version: "v0.2.0", url: "https://example.com/rel" },
          onOpenURL,
          onCheckUpdates: vi.fn(),
        }}
      />,
    );
    expect(screen.getAllByText("Version v0.2.0 is available.").length).toBeGreaterThan(0);
    fireEvent.click(screen.getAllByLabelText("Download update")[0]);
    expect(onOpenURL).toHaveBeenCalledWith("https://example.com/rel");
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
});

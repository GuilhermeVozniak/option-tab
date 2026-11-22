import { fireEvent, render, screen, within } from "@testing-library/react";
import { type ComponentProps, useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { makeT } from "../lib/i18n";
import type { LauncherStatus, ReplacementDockSettings } from "../lib/types";
import type { LauncherItemSettingsActions } from "./LauncherItems";
import { ReplacementDock } from "./ReplacementDock";

const value: ReplacementDockSettings = {
  version: 2,
  enabled: false,
  profiles: [
    {
      id: "default",
      name: "Default",
      edge: "bottom",
      layout: "floating",
      alignment: "center",
      appearance: {
        theme: "system",
        material: "solid",
        tint: "#172033",
        opacity: 0.76,
        borderOpacity: 0.16,
        cornerRadiusPx: 18,
        itemSpacingPx: 6,
        showLabels: true,
      },
      iconPx: 40,
      thicknessPx: 64,
      maxLengthFraction: 0.8,
      insetPx: 16,
      autoHide: false,
      widgets: [
        {
          id: "clock",
          packageID: "org.optiontab.clock",
          digest: "sha256:09bcb4221f7ace94e21898b4e583f9db72be1be5f6524fbe9972af70726c9dc3",
          enabled: false,
          grants: [],
        },
      ],
    },
  ],
  bindings: [
    { id: "main", target: "main", displayUUID: "", profileID: "default" },
    { id: "studio", target: "display", displayUUID: "display-studio", profileID: "default" },
  ],
  rules: [],
};

function openSection(name: string, t = makeT("en")) {
  fireEvent.click(
    within(screen.getByRole("navigation", { name: t("Launcher sections") })).getByRole("button", {
      name: t(name),
    }),
  );
}

describe("ReplacementDock", () => {
  it("keeps a launcher item draft mounted when switching configuration sections", async () => {
    const itemActions: LauncherItemSettingsActions = {
      load: vi.fn().mockResolvedValue({
        profileID: "default",
        revision: "saved",
        items: [],
        references: [],
        iconIDs: [],
      }),
      save: vi.fn(),
      chooseReference: vi.fn(),
      relinkReference: vi.fn(),
      cancelSelection: vi.fn(),
      chooseIcon: vi.fn(),
      removeReference: vi.fn(),
      removeIcon: vi.fn(),
    };
    render(
      <ReplacementDock
        value={value}
        t={makeT("en")}
        onChange={() => {}}
        itemActions={itemActions}
      />,
    );
    const navigation = within(screen.getByRole("navigation", { name: "Launcher sections" }));
    expect(screen.getByRole("textbox", { name: "Profile name" })).toBeVisible();
    expect(screen.queryByRole("textbox", { name: "Link label" })).toBeNull();
    fireEvent.click(navigation.getByRole("button", { name: "Items" }));
    await screen.findByText("No launcher items yet.");
    const label = screen.getByRole("textbox", { name: "Link label" });
    fireEvent.change(label, { target: { value: "Research" } });
    fireEvent.click(navigation.getByRole("button", { name: "Widgets" }));
    expect(label).toBeInTheDocument();
    expect(label).not.toBeVisible();
    fireEvent.click(navigation.getByRole("button", { name: "Items" }));
    expect(screen.getByRole("textbox", { name: "Link label" })).toBe(label);
    expect(label).toHaveValue("Research");
  });

  it("translates runtime Space refusal and hides unknown backend details", () => {
    const status: LauncherStatus = {
      epoch: 1,
      revision: 1,
      enabled: true,
      status: "unavailable",
      reason: "spaceUnavailable",
      recoveryLatched: false,
      displays: [],
      clockPackageID: "",
      clockDigest: "",
    };
    const view = render(
      <ReplacementDock value={value} status={status} t={makeT("pt-BR")} onChange={() => {}} />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("O Dock está indisponível neste Espaço.");
    view.rerender(
      <ReplacementDock value={value} status={status} t={makeT("es")} onChange={() => {}} />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "El Dock no está disponible en este espacio.",
    );
    view.rerender(
      <ReplacementDock
        value={value}
        error="native recovery failed: NSInternalException"
        t={makeT("es")}
        onChange={() => {}}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Dock sustituto no disponible");
    expect(screen.getByRole("alert")).not.toHaveTextContent("NSInternalException");
  });

  it.each([
    "pt-BR",
    "es",
  ] as const)("localizes profile and interaction accessible names in %s", (language) => {
    const t = makeT(language);
    render(<ReplacementDock value={value} t={t} language={language} onChange={() => {}} />);
    for (const [role, key, section] of [
      ["textbox", "Profile name", "Profile"],
      ["combobox", "Edge", "Layout"],
      ["combobox", "Launcher theme", "Layout"],
      ["combobox", "Primary gesture action", "Interactions"],
      ["switch", "Precise trackpad scrolling", "Interactions"],
      ["switch", "Pinch gestures", "Interactions"],
      ["switch", "Keyboard navigation", "Interactions"],
    ] as const) {
      openSection(section, t);
      expect(t(key)).not.toBe(key);
      expect(screen.getByRole(role, { name: t(key) })).toBeInTheDocument();
    }
    openSection("Displays", t);
    expect(screen.getByRole("combobox", { name: "Perfil de main" })).toBeInTheDocument();
  });

  it("keeps launcher and clock independently opt in", () => {
    const onChange = vi.fn();
    render(<ReplacementDock value={value} t={makeT("en")} onChange={onChange} />);
    fireEvent.click(screen.getByRole("switch", { name: "Enable replacement Dock" }));
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ enabled: true }));
    expect(onChange.mock.calls[0][0].profiles[0].widgets[0].enabled).toBe(false);
  });

  it("binds profiles to main and stable display UUID targets", () => {
    render(<ReplacementDock value={value} t={makeT("en")} onChange={() => {}} />);
    openSection("Displays");
    expect(screen.getByText("Main display")).toBeVisible();
    expect(screen.getByText(/display-studio/)).toBeVisible();
    expect(screen.getAllByText("Default").length).toBeGreaterThanOrEqual(2);
  });

  it("keeps clock access explicit when enabling the catalog widget", () => {
    const onChange = vi.fn();
    render(
      <ReplacementDock
        value={value}
        t={makeT("en")}
        onChange={onChange}
        widgetCatalog={[
          {
            packageID: "org.optiontab.clock",
            digest: "a".repeat(64),
            version: "1.0.0",
            name: { en: "Clock" },
            description: { en: "Local time" },
            requiredCapabilities: ["clock.read"],
            optionalCapabilities: [],
            settings: [],
            builtin: true,
          },
        ]}
      />,
    );
    openSection("Widgets");
    fireEvent.click(screen.getByRole("checkbox", { name: "Enable Clock" }));
    const next = onChange.mock.calls[0][0];
    expect(next.profiles[0].widgets[0]).toMatchObject({
      enabled: true,
      grants: [],
    });
  });

  it("edits bounded profile geometry and adds a stable display binding", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <ReplacementDock
        value={value}
        t={makeT("en")}
        onChange={onChange}
        status={{
          epoch: 2,
          revision: 1,
          enabled: false,
          status: "disabled",
          reason: "",
          recoveryLatched: false,
          displays: [
            {
              uuid: "display-main",
              name: "Built-in",
              main: true,
              bindingID: "main",
              profileID: "default",
              spaceKind: "unknown",
              status: "ready",
              reason: "",
            },
            {
              uuid: "display-projector",
              name: "Projector",
              main: false,
              bindingID: "",
              profileID: "",
              spaceKind: "unknown",
              status: "ready",
              reason: "",
            },
          ],
          clockPackageID: "org.optiontab.clock",
          clockDigest: "digest-final",
        }}
      />,
    );
    openSection("Layout");
    fireEvent.change(screen.getByLabelText("Icon size"), { target: { value: "52" } });
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ profiles: [expect.objectContaining({ iconPx: 52 })] }),
    );
    openSection("Displays");
    fireEvent.click(screen.getByRole("button", { name: "Add Projector" }));
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        bindings: expect.arrayContaining([
          expect.objectContaining({ target: "display", displayUUID: "display-projector" }),
        ]),
      }),
    );
  });

  it("keeps magnification scale and reach while the feature is disabled", () => {
    const onChange = vi.fn();
    const configured: ReplacementDockSettings = {
      ...value,
      profiles: [
        {
          ...value.profiles[0],
          magnification: { enabled: false, scale: 1.6, reach: 3 },
        },
      ],
    };
    const { rerender } = render(
      <ReplacementDock value={configured} t={makeT("en")} onChange={onChange} />,
    );
    openSection("Layout");
    expect(screen.getByLabelText("Enable launcher magnification")).not.toBeChecked();
    expect(screen.getByLabelText("Magnification scale")).toHaveValue(1.6);
    expect(screen.getByLabelText("Magnification reach")).toHaveValue(3);

    fireEvent.click(screen.getByLabelText("Enable launcher magnification"));
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        profiles: [
          expect.objectContaining({
            magnification: { enabled: true, scale: 1.6, reach: 3 },
          }),
        ],
      }),
    );
    const enabled = onChange.mock.calls.at(-1)?.[0] as ReplacementDockSettings;
    rerender(<ReplacementDock value={enabled} t={makeT("en")} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Magnification scale"), { target: { value: "1.8" } });
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        profiles: [
          expect.objectContaining({ magnification: { enabled: true, scale: 1.8, reach: 3 } }),
        ],
      }),
    );
    fireEvent.change(screen.getByLabelText("Magnification reach"), { target: { value: "0" } });
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        profiles: [
          expect.objectContaining({ magnification: { enabled: true, scale: 1.6, reach: 0 } }),
        ],
      }),
    );
  });

  it("uses off defaults when a legacy profile has no magnification object", () => {
    const onChange = vi.fn();
    render(<ReplacementDock value={value} t={makeT("en")} onChange={onChange} />);
    openSection("Layout");
    expect(screen.getByLabelText("Enable launcher magnification")).not.toBeChecked();
    expect(screen.getByLabelText("Magnification scale")).toHaveValue(1.35);
    expect(screen.getByLabelText("Magnification reach")).toHaveValue(2);
    fireEvent.click(screen.getByLabelText("Enable launcher magnification"));
    expect(onChange.mock.calls.at(-1)?.[0].profiles[0].magnification).toEqual({
      enabled: true,
      scale: 1.35,
      reach: 2,
    });
  });

  it.each([
    ["pt-BR" as const, "Ativar ampliação", "Escala de ampliação"],
    ["es" as const, "Activar ampliación", "Escala de ampliación"],
  ])("renders localized magnification controls in %s", (language, enable, scale) => {
    render(<ReplacementDock value={value} t={makeT(language)} onChange={() => {}} />);
    openSection("Layout", makeT(language));
    expect(screen.getByText(enable)).toBeVisible();
    expect(screen.getByText(scale)).toBeVisible();
  });

  it("keeps launcher interactions off and unavailable capabilities honest", () => {
    const onChange = vi.fn();
    render(<ReplacementDock value={value} t={makeT("en")} onChange={onChange} />);
    openSection("Interactions");
    expect(screen.getByLabelText("Enable launcher interactions")).not.toBeChecked();
    expect(screen.getByLabelText("Precise trackpad scrolling")).toBeDisabled();
    expect(screen.getByLabelText("Pinch gestures")).toBeDisabled();
    expect(screen.getByLabelText("Swipe gestures")).toBeDisabled();
    expect(screen.getByLabelText("Keyboard navigation")).toBeDisabled();
    expect(screen.getAllByText(/Unavailable on this device/)).toHaveLength(5);
    fireEvent.click(screen.getByLabelText("Enable launcher interactions"));
    expect(onChange.mock.calls.at(-1)?.[0].profiles[0].interactions).toEqual({
      enabled: true,
      preciseScroll: false,
      pinch: false,
      swipe: false,
      primaryAction: "next",
      towardAction: "showPreview",
      pinchAction: "showPreview",
      haptics: false,
      letterNavigation: false,
      enterActivates: false,
    });
  });

  it("preserves interaction choices while disabled and gates accepted capabilities", () => {
    const onChange = vi.fn();
    const configured: ReplacementDockSettings = {
      ...value,
      profiles: [
        {
          ...value.profiles[0],
          interactions: {
            enabled: true,
            preciseScroll: true,
            pinch: false,
            swipe: false,
            primaryAction: "previous",
            towardAction: "hidePreview",
            pinchAction: "showPreview",
            haptics: true,
            letterNavigation: true,
            enterActivates: true,
          },
        },
      ],
    };
    render(
      <ReplacementDock
        value={configured}
        t={makeT("en")}
        onChange={onChange}
        interactionCapabilities={{ preciseScroll: true, letterNavigation: true, haptics: true }}
      />,
    );
    openSection("Interactions");
    expect(screen.getByLabelText("Precise trackpad scrolling")).toBeEnabled();
    expect(screen.getByLabelText("Keyboard navigation")).toBeEnabled();
    expect(screen.getByLabelText("Pinch gestures")).toBeDisabled();
    expect(screen.getByLabelText("Launcher haptic feedback")).toBeEnabled();
    fireEvent.click(screen.getByLabelText("Enable launcher interactions"));
    expect(onChange.mock.calls.at(-1)?.[0].profiles[0].interactions).toMatchObject({
      enabled: false,
      preciseScroll: true,
      primaryAction: "previous",
      towardAction: "hidePreview",
      letterNavigation: true,
      enterActivates: true,
    });
  });

  it.each([
    ["pt-BR" as const, "Trackpad e teclado", "Indisponível neste dispositivo"],
    ["es" as const, "Trackpad y teclado", "No disponible en este dispositivo"],
  ])("renders localized interaction controls in %s", (language, heading, unavailable) => {
    render(<ReplacementDock value={value} t={makeT(language)} onChange={() => {}} />);
    openSection("Interactions", makeT(language));
    expect(screen.getByText(heading)).toBeVisible();
    expect(screen.getAllByText(new RegExp(unavailable))).toHaveLength(5);
  });

  it("offers permanent native Dock recovery and shows runtime refusal", () => {
    const recover = vi.fn();
    render(
      <ReplacementDock
        value={{ ...value, enabled: true }}
        t={makeT("en")}
        onChange={() => {}}
        onUseNativeDock={recover}
        status={{
          epoch: 4,
          revision: 1,
          enabled: false,
          status: "unavailable",
          reason: "spaceUnavailable",
          recoveryLatched: true,
          displays: [],
          clockPackageID: "org.optiontab.clock",
          clockDigest: "digest-final",
        }}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("The Dock is unavailable in this Space.");
    fireEvent.click(screen.getByRole("button", { name: "Use native Dock" }));
    expect(recover).toHaveBeenCalledOnce();
  });

  it("creates, renames, duplicates, and deletes profiles while reassigning bindings", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <ReplacementDock value={value} t={makeT("en")} onChange={onChange} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "New profile" }));
    const created = onChange.mock.calls.at(-1)?.[0];
    expect(created.profiles).toHaveLength(2);
    rerender(<ReplacementDock value={created} t={makeT("en")} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Profile"), {
      target: { value: created.profiles[1].id },
    });
    fireEvent.change(screen.getByLabelText("Profile name"), { target: { value: "Work" } });
    fireEvent.blur(screen.getByLabelText("Profile name"));
    expect(onChange.mock.calls.at(-1)?.[0].profiles[1].name).toBe("Work");
    fireEvent.click(screen.getByRole("button", { name: "Duplicate profile" }));
    expect(onChange.mock.calls.at(-1)?.[0].profiles).toHaveLength(3);
  });

  it("keeps a cleared profile name local until a valid rename is committed", () => {
    const onChange = vi.fn();
    render(<ReplacementDock value={value} t={makeT("en")} onChange={onChange} />);
    const name = screen.getByLabelText("Profile name");
    fireEvent.change(name, { target: { value: "" } });
    expect(name).toHaveValue("");
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.change(name, { target: { value: "Writing" } });
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.blur(name);
    expect(onChange).toHaveBeenCalledOnce();
    expect(onChange.mock.calls[0][0].profiles[0].name).toBe("Writing");
  });

  it("restores the saved profile name when an empty rename loses focus", () => {
    const onChange = vi.fn();
    render(<ReplacementDock value={value} t={makeT("en")} onChange={onChange} />);
    const name = screen.getByLabelText("Profile name");
    fireEvent.change(name, { target: { value: "" } });
    fireEvent.blur(name);
    expect(name).toHaveValue("Default");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("commits a profile name on Enter only once when focus subsequently leaves", () => {
    const onChange = vi.fn();
    render(<ReplacementDock value={value} t={makeT("en")} onChange={onChange} />);
    const name = screen.getByLabelText("Profile name");
    fireEvent.change(name, { target: { value: "Writing" } });
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.keyDown(name, { key: "Enter" });
    expect(onChange).toHaveBeenCalledOnce();
    expect(onChange.mock.calls[0][0].profiles[0].name).toBe("Writing");
    fireEvent.blur(name);
    expect(onChange).toHaveBeenCalledOnce();
  });

  it("cancels a draft profile name with Escape", () => {
    const onChange = vi.fn();
    render(<ReplacementDock value={value} t={makeT("en")} onChange={onChange} />);
    const name = screen.getByLabelText("Profile name");
    fireEvent.change(name, { target: { value: "Discard this" } });
    fireEvent.keyDown(name, { key: "Escape" });
    expect(name).toHaveValue("Default");
    fireEvent.blur(name);
    expect(onChange).not.toHaveBeenCalled();
  });

  it.each(["Enter", "Escape"])("leaves %s to the active input method", (key) => {
    const onChange = vi.fn();
    render(<ReplacementDock value={value} t={makeT("en")} onChange={onChange} />);
    const name = screen.getByLabelText("Profile name");
    fireEvent.change(name, { target: { value: "書く" } });
    expect(fireEvent.keyDown(name, { key, isComposing: true })).toBe(true);
    expect(name).toHaveValue("書く");
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.keyDown(name, { key: "Enter", isComposing: false });
    expect(onChange).toHaveBeenCalledOnce();
    expect(onChange.mock.calls[0][0].profiles[0].name).toBe("書く");
  });

  it("leaves WebKit's composition-confirming Enter unhandled when isComposing is false", () => {
    const onChange = vi.fn();
    render(<ReplacementDock value={value} t={makeT("en")} onChange={onChange} />);
    const name = screen.getByLabelText("Profile name");
    fireEvent.change(name, { target: { value: "書く" } });
    expect(fireEvent.keyDown(name, { key: "Enter", isComposing: false, keyCode: 229 })).toBe(true);
    expect(name).toHaveValue("書く");
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.keyDown(name, { key: "Enter", isComposing: false, keyCode: 13 });
    expect(onChange).toHaveBeenCalledOnce();
    expect(onChange.mock.calls[0][0].profiles[0].name).toBe("書く");
  });

  it("recovers new-profile item controls when the saved launcher epoch arrives", async () => {
    const itemActions: LauncherItemSettingsActions = {
      load: vi
        .fn()
        .mockRejectedValueOnce(new Error("launcher items: profileMissing"))
        .mockResolvedValue({
          profileID: "default",
          revision: "saved",
          items: [],
          references: [],
          iconIDs: [],
        }),
      save: vi.fn(),
      chooseReference: vi.fn(),
      relinkReference: vi.fn(),
      cancelSelection: vi.fn(),
      chooseIcon: vi.fn(),
      removeReference: vi.fn(),
      removeIcon: vi.fn(),
    };
    const status: LauncherStatus = {
      epoch: 1,
      revision: 1,
      enabled: false,
      status: "disabled",
      reason: "",
      recoveryLatched: false,
      displays: [],
      clockPackageID: "org.optiontab.clock",
      clockDigest: "digest-final",
    };
    const { rerender } = render(
      <ReplacementDock
        value={value}
        t={makeT("en")}
        onChange={() => {}}
        itemActions={itemActions}
        status={status}
      />,
    );
    openSection("Items");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Save the profile before editing its items.",
    );
    expect(screen.getByRole("button", { name: "Add application" })).toBeDisabled();
    rerender(
      <ReplacementDock
        value={value}
        t={makeT("en")}
        onChange={() => {}}
        itemActions={itemActions}
        status={{ ...status, epoch: 2 }}
      />,
    );
    await screen.findByText("No launcher items yet.");
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByRole("button", { name: "Add application" })).toBeEnabled();
  });

  it("preserves a draft on unrelated refreshes and hydrates a newly selected profile", () => {
    const configured = {
      ...value,
      profiles: [...value.profiles, { ...value.profiles[0], id: "second", name: "Second" }],
    };
    const onChange = vi.fn();
    const { rerender } = render(
      <ReplacementDock value={configured} t={makeT("en")} onChange={onChange} />,
    );
    fireEvent.change(screen.getByLabelText("Profile name"), { target: { value: "Draft" } });
    rerender(
      <ReplacementDock
        value={{ ...configured, enabled: true }}
        t={makeT("en")}
        onChange={onChange}
        status={{
          epoch: 5,
          revision: 2,
          enabled: true,
          status: "ready",
          reason: "",
          recoveryLatched: false,
          displays: [],
          clockPackageID: "org.optiontab.clock",
          clockDigest: "digest-final",
        }}
      />,
    );
    expect(screen.getByLabelText("Profile name")).toHaveValue("Draft");
    fireEvent.change(screen.getByLabelText("Profile"), { target: { value: "second" } });
    expect(screen.getByLabelText("Profile name")).toHaveValue("Second");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("clears an invalid delete replacement when the selected profile changes", () => {
    const second = { ...value.profiles[0], id: "second", name: "Second" };
    render(
      <ReplacementDock
        value={{ ...value, profiles: [...value.profiles, second] }}
        t={makeT("en")}
        onChange={() => {}}
      />,
    );
    fireEvent.change(screen.getByLabelText("Reassign deleted profile to"), {
      target: { value: "second" },
    });
    expect(screen.getByRole("button", { name: "Delete profile" })).toBeEnabled();
    fireEvent.change(screen.getByLabelText("Profile"), { target: { value: "second" } });
    expect(screen.getByRole("button", { name: "Delete profile" })).toBeDisabled();
  });

  it("bounds duplicated Unicode profile names to 80 characters", () => {
    const onChange = vi.fn();
    render(
      <ReplacementDock
        value={{ ...value, profiles: [{ ...value.profiles[0], name: "😀".repeat(80) }] }}
        t={makeT("en")}
        onChange={onChange}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Duplicate profile" }));
    expect(Array.from(onChange.mock.calls[0][0].profiles[1].name)).toHaveLength(80);
  });

  it("creates an exact app rule from a running-app suggestion and preserves its priority", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <ReplacementDock
        value={value}
        t={makeT("en")}
        onChange={onChange}
        appChoices={[
          { name: "Editor", bundleID: "com.example.editor" },
          { name: "Editor", bundleID: "org.example.editor" },
        ]}
      />,
    );
    openSection("Focus rules");
    fireEvent.click(screen.getByRole("button", { name: "Add focus rule" }));
    // A rule without a bundle identifier is never persisted.
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText("Priority 1")).toBeVisible();
    expect(screen.getByLabelText("Exact bundle identifier New rule")).toHaveValue("");

    fireEvent.change(screen.getByLabelText("Running app"), {
      target: { value: "org.example.editor" },
    });
    expect(onChange).toHaveBeenCalledTimes(1);
    const created = onChange.mock.calls.at(-1)?.[0];
    expect(created.rules).toEqual([
      {
        id: "focus-1",
        enabled: true,
        bundleID: "org.example.editor",
        profileID: "default",
        bindingID: "",
      },
    ]);

    rerender(
      <ReplacementDock
        value={created}
        t={makeT("en")}
        onChange={onChange}
        appChoices={[
          { name: "Editor", bundleID: "com.example.editor" },
          { name: "Editor", bundleID: "org.example.editor" },
        ]}
      />,
    );
    expect(screen.getByText("Priority 1")).toBeVisible();
    expect(screen.getByLabelText("Running app")).toHaveValue("org.example.editor");
    expect(screen.getByLabelText("Exact bundle identifier org.example.editor")).toHaveValue(
      "org.example.editor",
    );
  });

  it("edits, scopes, disables, reorders, and removes focus rules", () => {
    const onChange = vi.fn();
    const rules = [
      {
        id: "rule-a",
        enabled: true,
        bundleID: "com.example.a",
        profileID: "default",
        bindingID: "",
      },
      {
        id: "rule-b",
        enabled: true,
        bundleID: "com.example.b",
        profileID: "default",
        bindingID: "studio",
      },
    ];
    render(<ReplacementDock value={{ ...value, rules }} t={makeT("en")} onChange={onChange} />);
    openSection("Focus rules");
    fireEvent.click(screen.getByRole("switch", { name: "Enable rule com.example.a" }));
    expect(onChange.mock.calls.at(-1)?.[0].rules[0].enabled).toBe(false);
    fireEvent.change(screen.getByLabelText("Display scope com.example.a"), {
      target: { value: "studio" },
    });
    expect(onChange.mock.calls.at(-1)?.[0].rules[0].bindingID).toBe("studio");
    fireEvent.click(screen.getByRole("button", { name: "Move com.example.b up" }));
    expect(onChange.mock.calls.at(-1)?.[0].rules.map((rule: { id: string }) => rule.id)).toEqual([
      "rule-b",
      "rule-a",
    ]);
    fireEvent.click(screen.getByRole("button", { name: "Remove com.example.a" }));
    expect(onChange.mock.calls.at(-1)?.[0].rules).toEqual([rules[1]]);
  });

  it("reassigns deleted profile rules and drops only rules scoped to a removed binding", () => {
    const second = { ...value.profiles[0], id: "second", name: "Second" };
    const onChange = vi.fn();
    const withRules: ReplacementDockSettings = {
      ...value,
      profiles: [...value.profiles, second],
      rules: [
        {
          id: "global",
          enabled: true,
          bundleID: "com.example.global",
          profileID: "second",
          bindingID: "",
        },
        {
          id: "scoped",
          enabled: true,
          bundleID: "com.example.scoped",
          profileID: "default",
          bindingID: "studio",
        },
      ],
    };
    const { rerender } = render(
      <ReplacementDock value={withRules} t={makeT("en")} onChange={onChange} />,
    );
    fireEvent.change(screen.getByLabelText("Profile"), { target: { value: "second" } });
    fireEvent.change(screen.getByLabelText("Reassign deleted profile to"), {
      target: { value: "default" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Delete profile" }));
    expect(onChange.mock.calls.at(-1)?.[0].rules[0].profileID).toBe("default");

    rerender(<ReplacementDock value={withRules} t={makeT("en")} onChange={onChange} />);
    openSection("Displays");
    fireEvent.click(screen.getByRole("button", { name: "Remove assignment studio" }));
    expect(onChange.mock.calls.at(-1)?.[0].rules).toEqual([withRules.rules?.[0]]);
    expect(screen.getByText(/removes focus rules scoped only to that display/i)).toBeVisible();
  });

  it("shows invalid exact bundle identifiers without changing their meaning", () => {
    render(
      <ReplacementDock
        value={{
          ...value,
          rules: [
            {
              id: "invalid",
              enabled: true,
              bundleID: "Editor App",
              profileID: "default",
              bindingID: "",
            },
          ],
        }}
        t={makeT("en")}
        onChange={() => {}}
      />,
    );
    openSection("Focus rules");
    expect(screen.getByRole("alert")).toHaveTextContent("Enter an exact bundle identifier");
    expect(screen.getByLabelText("Exact bundle identifier Editor App")).toHaveValue("Editor App");
  });
});

it("runtime reordering is opt-in and updates only the selected profile", () => {
  const onChange = vi.fn();
  render(<ReplacementDock value={value} onChange={onChange} t={(s) => s} />);
  openSection("Items");
  const control = screen.getByLabelText("Enable runtime launcher reordering");
  expect(control).not.toBeChecked();
  fireEvent.click(control);
  expect(onChange.mock.calls.at(-1)?.[0].profiles[0].runtimeReorder).toBe(true);
  expect(onChange.mock.calls.at(-1)?.[0].enabled).toBe(false);
});

it("Dock badge observation is opt-in and keeps the Dock disabled", () => {
  const onChange = vi.fn();
  render(<ReplacementDock value={value} onChange={onChange} t={(s) => s} />);
  openSection("Items");
  const control = screen.getByLabelText("Show replacement Dock badges");
  expect(control).not.toBeChecked();
  fireEvent.click(control);
  expect(onChange.mock.calls.at(-1)?.[0].profiles[0].showBadges).toBe(true);
  expect(onChange.mock.calls.at(-1)?.[0].enabled).toBe(false);
});

// Mirrors config.ValidateReplacementDock (replacement.go): the launcher subtree
// is rejected, not clamped, when any field or relationship is invalid.
function goAccepts(s: ReplacementDockSettings): boolean {
  const id = /^[a-z0-9][a-z0-9._-]{0,63}$/;
  const whole = (n: number, lo: number, hi: number) => Number.isInteger(n) && n >= lo && n <= hi;
  const real = (n: number, lo: number, hi: number) => Number.isFinite(n) && n >= lo && n <= hi;
  const rules = s.rules ?? [];
  if (!Array.isArray(s.profiles) || !Array.isArray(s.bindings)) return false;
  if (s.version !== 2 || s.profiles.length === 0 || s.profiles.length > 8) return false;
  if (s.bindings.length > 8 || rules.length > 8) return false;
  const profiles = new Set<string>();
  for (const p of s.profiles) {
    const a = p.appearance;
    const m = p.magnification;
    if (!id.test(p.id) || profiles.has(p.id) || Array.from(p.name).length < 1) return false;
    if (!whole(p.iconPx, 24, 64) || !whole(p.thicknessPx, 48, 112)) return false;
    if (p.thicknessPx < p.iconPx + 8) return false;
    if (!real(p.maxLengthFraction, 0.25, 0.9) || !whole(p.insetPx, 12, 64)) return false;
    if (!real(a.opacity, 0.35, 1) || !real(a.borderOpacity, 0, 0.5)) return false;
    if (!whole(a.cornerRadiusPx, 0, 28) || !whole(a.itemSpacingPx, 2, 20)) return false;
    if (m && (!real(m.scale, 1, 2) || !whole(m.reach, 0, 4))) return false;
    if (!Array.isArray(p.widgets) || p.widgets.length > 16) return false;
    profiles.add(p.id);
  }
  const bindings = new Set<string>();
  const targets = new Set<string>();
  for (const b of s.bindings) {
    if (!id.test(b.id) || bindings.has(b.id) || !profiles.has(b.profileID)) return false;
    if (b.target === "main" ? b.displayUUID !== "" : b.displayUUID === "") return false;
    const key = `${b.target}:${b.displayUUID}`;
    if (targets.has(key)) return false;
    targets.add(key);
    bindings.add(b.id);
  }
  const ruleIDs = new Set<string>();
  for (const r of rules) {
    if (!id.test(r.id) || ruleIDs.has(r.id) || !/^[A-Za-z0-9._-]{1,255}$/.test(r.bundleID))
      return false;
    if (!profiles.has(r.profileID) || (r.bindingID !== "" && !bindings.has(r.bindingID)))
      return false;
    ruleIDs.add(r.id);
  }
  return true;
}

// Stands in for the save round trip: a valid document is stored and echoed
// back, an invalid one is rejected and the canonical value stays.
function Backend({
  initial,
  saved,
  rejected,
  ...props
}: {
  initial: ReplacementDockSettings;
  saved: (value: ReplacementDockSettings) => void;
  rejected: (value: ReplacementDockSettings) => void;
} & Omit<ComponentProps<typeof ReplacementDock>, "value" | "t" | "onChange">) {
  const [current, setCurrent] = useState(initial);
  return (
    <ReplacementDock
      {...props}
      value={current}
      t={makeT("en")}
      onChange={(next) => {
        if (!goAccepts(next)) {
          rejected(next);
          return;
        }
        saved(next);
        setCurrent(next);
      }}
    />
  );
}

function backend(
  initial: ReplacementDockSettings = value,
  props: Omit<ComponentProps<typeof ReplacementDock>, "value" | "t" | "onChange"> = {},
) {
  const saved = vi.fn<(value: ReplacementDockSettings) => void>();
  const rejected = vi.fn<(value: ReplacementDockSettings) => void>();
  render(<Backend initial={initial} saved={saved} rejected={rejected} {...props} />);
  const last = () => saved.mock.calls.at(-1)?.[0] as ReplacementDockSettings;
  return { saved, rejected, last };
}

// The first keystroke replaces the selected text; later ones append to
// whatever the field shows after the previous save was echoed back.
function typeOver(input: HTMLElement, text: string) {
  const field = input as HTMLInputElement;
  // jsdom sanitizes a half-typed fraction ("0.") to "", so fractions arrive whole.
  if (text.includes(".")) {
    fireEvent.input(field, { target: { value: text }, inputType: "insertText" });
    return;
  }
  const [first, ...rest] = Array.from(text);
  fireEvent.input(field, { target: { value: first }, inputType: "insertText" });
  for (const character of rest) {
    fireEvent.input(field, { target: { value: field.value + character }, inputType: "insertText" });
  }
}

function displayStatus(...names: string[]): LauncherStatus {
  return {
    epoch: 2,
    revision: 1,
    enabled: false,
    status: "disabled",
    reason: "",
    recoveryLatched: false,
    displays: names.map((name, index) => ({
      uuid: `display-${name.toLowerCase()}`,
      name,
      main: index === 0,
      bindingID: "",
      profileID: "",
      spaceKind: "unknown",
      status: "ready",
      reason: "",
    })),
    clockPackageID: "org.optiontab.clock",
    clockDigest: "digest-final",
  };
}

describe("ReplacementDock against the validating backend", () => {
  it("types an icon size without submitting the partial digits", () => {
    const { saved, rejected, last } = backend();
    openSection("Layout");
    const icon = screen.getByLabelText("Icon size");
    typeOver(icon, "48");
    expect(saved).not.toHaveBeenCalled();
    fireEvent.blur(icon);
    expect(rejected).not.toHaveBeenCalled();
    expect(saved).toHaveBeenCalledTimes(1);
    expect(last().profiles[0].iconPx).toBe(48);
    expect(icon).toHaveValue(48);
  });

  it.each([
    ["Icon size", "30", 30, "9", 24],
    ["Maximum length", "0.5", 0.5, "4", 0.9],
    ["Dock thickness", "90", 90, "500", 112],
    ["Screen inset", "24", 24, "1", 12],
    ["Launcher opacity", "0.9", 0.9, "0.1", 0.35],
    ["Border opacity", "0.3", 0.3, "7", 0.5],
    ["Corner radius", "10", 10, "99", 28],
    ["Item spacing", "12", 12, "1", 2],
    ["Magnification scale", "1.8", 1.8, "9", 2],
    ["Magnification reach", "4", 4, "12", 4],
  ] as const)("commits %s on Enter, clamps it, and never saves an emptied field", (label, typed, expected, outside, bound) => {
    const { saved, rejected } = backend({
      ...value,
      profiles: [{ ...value.profiles[0], magnification: { enabled: true, scale: 1.35, reach: 2 } }],
    });
    openSection("Layout");
    const input = screen.getByLabelText(label);
    typeOver(input, typed);
    expect(saved).not.toHaveBeenCalled();
    fireEvent.keyDown(input, { key: "Enter" });
    expect(saved).toHaveBeenCalledTimes(1);
    expect(input).toHaveValue(expected);
    typeOver(input, outside);
    fireEvent.blur(input);
    expect(input).toHaveValue(bound);
    const saves = saved.mock.calls.length;
    fireEvent.input(input, { target: { value: "" }, inputType: "deleteContentBackward" });
    fireEvent.blur(input);
    expect(saved).toHaveBeenCalledTimes(saves);
    expect(input).toHaveValue(bound);
    expect(rejected).not.toHaveBeenCalled();
  });

  it("keeps icon size and Dock thickness in a combination the backend accepts", () => {
    const { rejected, last } = backend();
    openSection("Layout");
    const icon = screen.getByLabelText("Icon size");
    const thickness = screen.getByLabelText("Dock thickness");
    expect(thickness).toHaveValue(64);
    // Stepper: 56 is the largest icon that fits a thickness of 64.
    fireEvent.change(icon, { target: { value: "56" } });
    expect(last().profiles[0]).toMatchObject({ iconPx: 56, thicknessPx: 64 });
    fireEvent.change(icon, { target: { value: "57" } });
    expect(last().profiles[0]).toMatchObject({ iconPx: 57, thicknessPx: 65 });
    expect(thickness).toHaveValue(65);
    // Every valid icon size stays reachable in one edit.
    typeOver(icon, "64");
    fireEvent.blur(icon);
    expect(last().profiles[0]).toMatchObject({ iconPx: 64, thicknessPx: 72 });
    // Lowering the thickness brings the icon down with it.
    typeOver(thickness, "48");
    fireEvent.keyDown(thickness, { key: "Enter" });
    expect(last().profiles[0]).toMatchObject({ iconPx: 40, thicknessPx: 48 });
    expect(icon).toHaveValue(40);
    typeOver(icon, "24");
    fireEvent.blur(icon);
    expect(last().profiles[0]).toMatchObject({ iconPx: 24, thicknessPx: 48 });
    typeOver(thickness, "112");
    fireEvent.blur(thickness);
    expect(last().profiles[0]).toMatchObject({ iconPx: 24, thicknessPx: 112 });
    expect(rejected).not.toHaveBeenCalled();
  });

  it("keeps a new focus rule local until it has a valid bundle identifier", () => {
    const { saved, rejected, last } = backend();
    openSection("Focus rules");
    const add = screen.getByRole("button", { name: "Add focus rule" });
    fireEvent.click(add);
    expect(saved).not.toHaveBeenCalled();
    expect(rejected).not.toHaveBeenCalled();
    expect(add).toBeDisabled();
    expect(screen.queryByText("No focus rules")).toBeNull();
    const bundle = screen.getByLabelText("Exact bundle identifier New rule");

    fireEvent.change(bundle, { target: { value: "Editor App" } });
    fireEvent.blur(bundle);
    expect(saved).not.toHaveBeenCalled();
    expect(rejected).not.toHaveBeenCalled();
    expect(bundle).toHaveValue("Editor App");
    expect(screen.getByRole("alert")).toHaveTextContent("Enter an exact bundle identifier");

    fireEvent.click(screen.getByRole("switch", { name: "Enable rule New rule" }));
    expect(saved).not.toHaveBeenCalled();

    fireEvent.change(bundle, { target: { value: " com.example.editor " } });
    expect(saved).not.toHaveBeenCalled();
    fireEvent.keyDown(bundle, { key: "Enter" });
    expect(rejected).not.toHaveBeenCalled();
    expect(saved).toHaveBeenCalledTimes(1);
    expect(last().rules).toEqual([
      {
        id: "focus-1",
        enabled: false,
        bundleID: "com.example.editor",
        profileID: "default",
        bindingID: "",
      },
    ]);
    expect(screen.getByLabelText("Exact bundle identifier com.example.editor")).toHaveValue(
      "com.example.editor",
    );
    expect(screen.queryByRole("alert")).toBeNull();
    expect(add).toBeEnabled();
  });

  it("discards an unfinished focus rule without saving", () => {
    const { saved, rejected } = backend();
    openSection("Focus rules");
    fireEvent.click(screen.getByRole("button", { name: "Add focus rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Remove New rule" }));
    expect(screen.getByText("No focus rules")).toBeVisible();
    expect(screen.getByRole("button", { name: "Add focus rule" })).toBeEnabled();
    expect(saved).not.toHaveBeenCalled();
    expect(rejected).not.toHaveBeenCalled();
  });

  it("keeps bundle identifier edits local until they are valid", () => {
    const rule = {
      id: "focus-1",
      enabled: true,
      bundleID: "com.example.a",
      profileID: "default",
      bindingID: "",
    };
    const { saved, rejected, last } = backend(
      { ...value, rules: [rule] },
      { appChoices: [{ name: "A", bundleID: "com.example.a" }] },
    );
    openSection("Focus rules");
    const bundle = screen.getByLabelText("Exact bundle identifier com.example.a");

    fireEvent.change(bundle, { target: { value: "" } });
    fireEvent.blur(bundle);
    expect(bundle).toHaveValue("");
    expect(screen.getByRole("alert")).toHaveTextContent("Enter an exact bundle identifier");
    fireEvent.change(bundle, { target: { value: "com.example/b" } });
    fireEvent.keyDown(bundle, { key: "Enter" });
    expect(saved).not.toHaveBeenCalled();
    expect(rejected).not.toHaveBeenCalled();

    fireEvent.keyDown(bundle, { key: "Escape" });
    expect(bundle).toHaveValue("com.example.a");
    expect(screen.queryByRole("alert")).toBeNull();

    // Choosing "Enter bundle identifier" clears the field without saving "".
    fireEvent.change(screen.getByLabelText("Running app"), { target: { value: "" } });
    expect(bundle).toHaveValue("");
    expect(saved).not.toHaveBeenCalled();
    expect(rejected).not.toHaveBeenCalled();

    fireEvent.change(bundle, { target: { value: "com.example.b" } });
    expect(saved).not.toHaveBeenCalled();
    fireEvent.blur(bundle);
    expect(rejected).not.toHaveBeenCalled();
    expect(saved).toHaveBeenCalledTimes(1);
    expect(last().rules).toEqual([{ ...rule, bundleID: "com.example.b" }]);
    expect(screen.getByLabelText("Exact bundle identifier com.example.b")).toHaveValue(
      "com.example.b",
    );
  });

  it("gives a display binding added after a removal a free id", () => {
    const { rejected, last } = backend(
      { ...value, bindings: [value.bindings[0]] },
      { status: displayStatus("Built-in", "Projector", "Studio") },
    );
    openSection("Displays");
    fireEvent.click(screen.getByRole("button", { name: "Add Projector" }));
    expect(last().bindings.map((binding) => binding.id)).toEqual(["main", "display-2"]);
    fireEvent.click(screen.getByRole("button", { name: "Remove assignment main" }));
    expect(last().bindings.map((binding) => binding.id)).toEqual(["display-2"]);
    fireEvent.click(screen.getByRole("button", { name: "Add Studio" }));
    expect(rejected).not.toHaveBeenCalled();
    expect(last().bindings.map((binding) => binding.id)).toEqual(["display-2", "display-3"]);
  });

  it("renders a profile whose widgets and bindings arrived as null", () => {
    const broken = {
      ...value,
      bindings: null,
      profiles: [{ ...value.profiles[0], widgets: null }],
    } as unknown as ReplacementDockSettings;
    const saved = vi.fn<(value: ReplacementDockSettings) => void>();
    render(
      <ReplacementDock
        value={broken}
        t={makeT("en")}
        onChange={saved}
        status={displayStatus("Built-in")}
      />,
    );
    for (const section of ["Widgets", "Displays", "Focus rules", "Profile"]) openSection(section);
    expect(screen.getByRole("textbox", { name: "Profile name" })).toHaveValue("Default");
    fireEvent.click(screen.getByRole("button", { name: "Duplicate profile" }));
    expect(saved.mock.calls.at(-1)?.[0].profiles[1].widgets).toEqual([]);
    openSection("Displays");
    fireEvent.click(screen.getByRole("button", { name: "Add Built-in" }));
    expect(saved.mock.calls.at(-1)?.[0].bindings).toEqual([
      { id: "display-1", target: "main", displayUUID: "", profileID: "default" },
    ]);
    openSection("Focus rules");
    fireEvent.click(screen.getByRole("button", { name: "Add focus rule" }));
    expect(screen.getByLabelText("Display scope New rule")).toBeInTheDocument();
  });

  it("renders a widget whose grants arrived as null when duplicating its profile", () => {
    const broken = {
      ...value,
      profiles: [
        { ...value.profiles[0], widgets: [{ ...value.profiles[0].widgets[0], grants: null }] },
      ],
    } as unknown as ReplacementDockSettings;
    const saved = vi.fn<(value: ReplacementDockSettings) => void>();
    render(
      <ReplacementDock
        value={broken}
        t={makeT("en")}
        onChange={saved}
        widgetCatalog={[
          {
            packageID: "org.optiontab.clock",
            digest: "a".repeat(64),
            version: "1.0.0",
            name: { en: "Clock" },
            description: { en: "Local time" },
            requiredCapabilities: ["clock.read"],
            optionalCapabilities: [],
            settings: [],
            builtin: true,
          },
        ]}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "New profile" }));
    expect(saved.mock.calls.at(-1)?.[0].profiles[1].widgets[0].grants).toEqual([]);
    openSection("Widgets");
    fireEvent.click(screen.getByRole("checkbox", { name: "Enable Clock" }));
    expect(saved.mock.calls.at(-1)?.[0].profiles[0].widgets[0]).toMatchObject({
      enabled: true,
      grants: [],
    });
  });
});

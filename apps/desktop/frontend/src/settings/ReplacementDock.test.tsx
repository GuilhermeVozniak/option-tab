import { fireEvent, render, screen } from "@testing-library/react";
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

describe("ReplacementDock", () => {
  it("keeps launcher and clock independently opt in", () => {
    const onChange = vi.fn();
    render(<ReplacementDock value={value} t={makeT("en")} onChange={onChange} />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Enable replacement Dock" }));
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ enabled: true }));
    expect(onChange.mock.calls[0][0].profiles[0].widgets[0].enabled).toBe(false);
  });

  it("binds profiles to main and stable display UUID targets", () => {
    render(<ReplacementDock value={value} t={makeT("en")} onChange={() => {}} />);
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
    fireEvent.change(screen.getByLabelText("Icon size"), { target: { value: "52" } });
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ profiles: [expect.objectContaining({ iconPx: 52 })] }),
    );
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
    expect(screen.getByText(enable)).toBeVisible();
    expect(screen.getByText(scale)).toBeVisible();
  });

  it("keeps launcher interactions off and unavailable capabilities honest", () => {
    const onChange = vi.fn();
    render(<ReplacementDock value={value} t={makeT("en")} onChange={onChange} />);
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
    expect(screen.getByRole("alert")).toHaveTextContent("spaceUnavailable");
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
    expect(await screen.findByRole("alert")).toHaveTextContent("profileMissing");
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
    fireEvent.click(screen.getByRole("button", { name: "Add focus rule" }));
    const created = onChange.mock.calls.at(-1)?.[0];
    expect(created.rules).toEqual([
      expect.objectContaining({ enabled: true, bundleID: "", profileID: "default" }),
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
    fireEvent.change(screen.getByLabelText("Running app"), {
      target: { value: "org.example.editor" },
    });
    expect(onChange.mock.calls.at(-1)?.[0].rules[0].bundleID).toBe("org.example.editor");
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
    fireEvent.click(screen.getByRole("checkbox", { name: "Enable rule com.example.a" }));
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
    expect(screen.getByRole("alert")).toHaveTextContent("Enter an exact bundle identifier");
    expect(screen.getByLabelText("Exact bundle identifier Editor App")).toHaveValue("Editor App");
  });
});

it("runtime reordering is opt-in and updates only the selected profile", () => {
  const onChange = vi.fn();
  render(<ReplacementDock value={value} onChange={onChange} t={(s) => s} />);
  const control = screen.getByLabelText("Enable runtime launcher reordering");
  expect(control).not.toBeChecked();
  fireEvent.click(control);
  expect(onChange.mock.calls.at(-1)?.[0].profiles[0].runtimeReorder).toBe(true);
  expect(onChange.mock.calls.at(-1)?.[0].enabled).toBe(false);
});

it("Dock badge observation is opt-in and keeps the Dock disabled", () => {
  const onChange = vi.fn();
  render(<ReplacementDock value={value} onChange={onChange} t={(s) => s} />);
  const control = screen.getByLabelText("Show replacement Dock badges");
  expect(control).not.toBeChecked();
  fireEvent.click(control);
  expect(onChange.mock.calls.at(-1)?.[0].profiles[0].showBadges).toBe(true);
  expect(onChange.mock.calls.at(-1)?.[0].enabled).toBe(false);
});

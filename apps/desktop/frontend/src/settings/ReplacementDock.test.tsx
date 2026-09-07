import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { makeT } from "../lib/i18n";
import type { ReplacementDockSettings } from "../lib/types";
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
          digest: "builtin-clock-v1",
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

  it("grants clock.read only with an explicit clock enable", () => {
    const onChange = vi.fn();
    render(<ReplacementDock value={value} t={makeT("en")} onChange={onChange} />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Show clock" }));
    const next = onChange.mock.calls[0][0];
    expect(next.profiles[0].widgets[0]).toMatchObject({
      enabled: true,
      grants: ["clock.read"],
    });
  });

  it("edits bounded profile geometry and adds a stable display binding", () => {
    const onChange = vi.fn();
    render(
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
    expect(onChange.mock.calls.at(-1)?.[0].profiles[1].name).toBe("Work");
    fireEvent.click(screen.getByRole("button", { name: "Duplicate profile" }));
    expect(onChange.mock.calls.at(-1)?.[0].profiles).toHaveLength(3);
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
});

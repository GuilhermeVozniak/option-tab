import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type {
  WidgetCatalogDescriptor,
  WidgetInstanceConfig,
  WidgetStackConfig,
} from "../lib/widget-types";
import { WidgetSettings } from "./WidgetSettings";

const clock: WidgetCatalogDescriptor = {
  packageID: "org.optiontab.clock",
  digest: "a".repeat(64),
  version: "1.0.0",
  name: { en: "Clock", "pt-BR": "Relógio", es: "Reloj" },
  description: { en: "Local time", "pt-BR": "Hora local", es: "Hora local" },
  requiredCapabilities: ["clock.read"],
  optionalCapabilities: [],
  settings: [
    {
      id: "format",
      type: "choice",
      name: { en: "Format", "pt-BR": "Formato", es: "Formato" },
      defaultText: "shortTime",
      options: ["shortTime", "longTime"],
    },
  ],
  builtin: true,
};
const network: WidgetCatalogDescriptor = {
  ...clock,
  packageID: "org.optiontab.network",
  digest: "b".repeat(64),
  name: { en: "Network" },
  requiredCapabilities: ["network.status.read"],
  optionalCapabilities: ["network.usage.read"],
  settings: [],
};
const renderEditor = (widgets: WidgetInstanceConfig[] = [], stacks: WidgetStackConfig[] = []) => {
  const onChange = vi.fn();
  render(
    <WidgetSettings
      instances={widgets}
      stacks={stacks}
      catalog={[clock, network]}
      onChange={onChange}
      language="en"
    />,
  );
  return onChange;
};

describe("WidgetSettings", () => {
  it("keeps a time-zone draft local while typing and commits its complete value on blur", () => {
    const onChange = vi.fn();
    render(
      <WidgetSettings
        instances={[
          {
            id: "clock",
            packageID: clock.packageID,
            digest: clock.digest,
            enabled: true,
            grants: ["clock.read"],
          },
        ]}
        stacks={[]}
        catalog={[
          {
            ...clock,
            settings: [
              { id: "timezone", type: "timezone", name: { en: "Time zone" }, defaultText: "Local" },
            ],
          },
        ]}
        onChange={onChange}
      />,
    );
    const field = screen.getByLabelText("Time zone");
    fireEvent.change(field, { target: { value: "" } });
    fireEvent.change(field, { target: { value: "E" } });
    fireEvent.change(field, { target: { value: "Europe/" } });
    fireEvent.change(field, { target: { value: "Europe/Rome" } });
    expect(onChange).not.toHaveBeenCalled();
    expect(field).toHaveValue("Europe/Rome");
    fireEvent.blur(field);
    expect(onChange).toHaveBeenCalledOnce();
    expect(onChange.mock.calls[0][0].widgets[0].settings).toEqual({
      timezone: { text: "Europe/Rome" },
    });
  });

  it("rejects an incomplete time zone locally and supports Enter and Escape without saving prefixes", () => {
    const onChange = vi.fn();
    render(
      <WidgetSettings
        instances={[
          {
            id: "clock",
            packageID: clock.packageID,
            digest: clock.digest,
            enabled: true,
            grants: ["clock.read"],
          },
        ]}
        stacks={[]}
        catalog={[
          {
            ...clock,
            settings: [
              { id: "timezone", type: "timezone", name: { en: "Time zone" }, defaultText: "Local" },
            ],
          },
        ]}
        onChange={onChange}
      />,
    );
    const field = screen.getByLabelText("Time zone");
    fireEvent.change(field, { target: { value: "Europe/" } });
    fireEvent.blur(field);
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("Enter a valid value.");
    fireEvent.keyDown(field, { key: "Escape" });
    expect(field).toHaveValue("Local");
    expect(screen.queryByRole("alert")).toBeNull();
    fireEvent.change(field, { target: { value: "UTC" } });
    fireEvent.keyDown(field, { key: "Enter", isComposing: true });
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.keyDown(field, { key: "Enter" });
    expect(onChange).toHaveBeenCalledOnce();
    expect(onChange.mock.calls[0][0].widgets[0].settings).toEqual({ timezone: { text: "UTC" } });
  });

  it.each([
    "+01:00",
    "-08:30",
  ])("rejects numeric time zone %s while accepting Etc/GMT+1", (zone) => {
    const onChange = vi.fn();
    render(
      <WidgetSettings
        instances={[
          {
            id: "clock",
            packageID: clock.packageID,
            digest: clock.digest,
            enabled: true,
            grants: ["clock.read"],
          },
        ]}
        stacks={[]}
        catalog={[
          {
            ...clock,
            settings: [
              { id: "timezone", type: "timezone", name: { en: "Time zone" }, defaultText: "Local" },
            ],
          },
        ]}
        onChange={onChange}
      />,
    );
    const field = screen.getByLabelText("Time zone");
    fireEvent.change(field, { target: { value: zone } });
    fireEvent.blur(field);
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("Enter a valid value.");
    fireEvent.change(field, { target: { value: "Etc/GMT+1" } });
    fireEvent.keyDown(field, { key: "Enter" });
    expect(onChange).toHaveBeenCalledOnce();
    expect(onChange.mock.calls[0][0].widgets[0].settings).toEqual({
      timezone: { text: "Etc/GMT+1" },
    });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("selects an exact package digest when multiple versions share an ID", () => {
    const newer = { ...clock, digest: "d".repeat(64), version: "2.0.0", builtin: false };
    const onChange = vi.fn();
    render(
      <WidgetSettings instances={[]} stacks={[]} catalog={[clock, newer]} onChange={onChange} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Add widget" }));
    expect(onChange.mock.calls.at(-1)?.[0].widgets[0].digest).toBe(clock.digest);
    const option = screen.getByRole("option", { name: "Clock · 2.0.0" }) as HTMLOptionElement;
    fireEvent.change(screen.getByLabelText("Widget package"), { target: { value: option.value } });
    fireEvent.click(screen.getByRole("button", { name: "Add widget" }));
    expect(onChange.mock.calls.at(-1)?.[0].widgets[0].digest).toBe(newer.digest);
  });

  it("uses the installed digest's settings and grants without borrowing another version", () => {
    const newer = {
      ...clock,
      digest: "d".repeat(64),
      version: "2.0.0",
      builtin: false,
      requiredCapabilities: ["battery.read"],
      settings: [],
    };
    render(
      <WidgetSettings
        instances={[
          {
            id: "clock",
            packageID: clock.packageID,
            digest: clock.digest,
            enabled: false,
            grants: [],
          },
        ]}
        stacks={[]}
        catalog={[clock, newer]}
        onChange={() => {}}
      />,
    );
    expect(screen.getByLabelText("Format")).toBeVisible();
    expect(screen.getByText("Required · Read local time")).toBeVisible();
    expect(screen.queryByText("Required · Read battery status")).not.toBeInTheDocument();
  });

  it("counts existing stacks when creating another stack", () => {
    const widgets = ["one", "two", "three", "four", "five", "six"].map((id) => ({
      id,
      packageID: clock.packageID,
      digest: clock.digest,
      enabled: false,
      grants: [],
    }));
    const onChange = renderEditor(widgets, [
      { id: "first", name: "First", members: ["one", "two", "three"], activeID: "one" },
    ]);
    fireEvent.click(screen.getByLabelText("Stack four"));
    fireEvent.click(screen.getByLabelText("Stack five"));
    expect(screen.getByRole("button", { name: "Create stack" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Create stack" }));
    expect(onChange.mock.calls.at(-1)?.[0].stacks).toHaveLength(2);
  });

  it("drops removed instances from a pending stack selection", () => {
    const widgets = ["one", "two"].map((id) => ({
      id,
      packageID: clock.packageID,
      digest: clock.digest,
      enabled: false,
      grants: [],
    }));
    const onChange = vi.fn();
    const { rerender } = render(
      <WidgetSettings instances={widgets} stacks={[]} catalog={[clock]} onChange={onChange} />,
    );
    fireEvent.click(screen.getByLabelText("Stack one"));
    fireEvent.click(screen.getByLabelText("Stack two"));
    rerender(
      <WidgetSettings instances={[widgets[0]]} stacks={[]} catalog={[clock]} onChange={onChange} />,
    );
    expect(screen.getByRole("button", { name: "Create stack" })).toBeDisabled();
  });

  it("adds a catalog widget disabled and without implicit grants", () => {
    const onChange = renderEditor();
    fireEvent.change(screen.getByLabelText("Widget package"), {
      target: { value: network.digest },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add widget" }));
    expect(onChange).toHaveBeenCalledWith({
      widgets: [
        expect.objectContaining({ packageID: network.packageID, enabled: false, grants: [] }),
      ],
      stacks: [],
    });
  });

  it("distinguishes required and optional capabilities and never grants on enable", () => {
    const instance = {
      id: "network",
      packageID: network.packageID,
      digest: network.digest,
      enabled: false,
      grants: [],
    };
    const onChange = renderEditor([instance]);
    expect(screen.getByText("Required · Network status")).toBeVisible();
    expect(screen.getByText("Optional · Network usage")).toBeVisible();
    expect(screen.getByText("Unavailable until required access is granted.")).toBeVisible();
    fireEvent.click(screen.getByLabelText("Enable Network"));
    expect(onChange).toHaveBeenLastCalledWith({
      widgets: [{ ...instance, enabled: true }],
      stacks: [],
    });
  });

  it("canonicalizes only the exact legacy clock when it is edited", () => {
    const legacy = {
      id: "clock",
      packageID: "org.optiontab.clock",
      digest: "sha256:09bcb4221f7ace94e21898b4e583f9db72be1be5f6524fbe9972af70726c9dc3",
      enabled: true,
      grants: ["clock.read"],
    };
    const onChange = renderEditor([legacy]);
    fireEvent.change(screen.getByLabelText("Format"), { target: { value: "longTime" } });
    expect(onChange).toHaveBeenLastCalledWith({
      widgets: [
        expect.objectContaining({
          digest: clock.digest,
          enabled: true,
          grants: ["clock.read"],
          settings: { format: { text: "longTime" } },
        }),
      ],
      stacks: [],
    });
  });

  it("preserves a nonmatching clock digest while editing ordinary fields", () => {
    const instance = {
      id: "clock",
      packageID: "org.optiontab.clock",
      digest: "c".repeat(64),
      enabled: false,
      grants: [],
    };
    const onChange = renderEditor([instance]);
    expect(screen.queryByLabelText("Format")).not.toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Enable org.optiontab.clock"));
    expect(onChange).toHaveBeenLastCalledWith({
      widgets: [{ ...instance, enabled: true }],
      stacks: [],
    });
  });

  it("removes stack references with an instance and preserves immutable inputs", () => {
    const widgets = [
      { id: "one", packageID: clock.packageID, digest: clock.digest, enabled: false, grants: [] },
      {
        id: "two",
        packageID: network.packageID,
        digest: network.digest,
        enabled: false,
        grants: [],
      },
    ];
    const stacks = [{ id: "pair", name: "Status", members: ["one", "two"], activeID: "one" }];
    const snapshot = structuredClone({ widgets, stacks });
    const onChange = renderEditor(widgets, stacks);
    fireEvent.click(screen.getByRole("button", { name: "Remove Clock" }));
    expect(onChange).toHaveBeenCalledWith({ widgets: [widgets[1]], stacks: [] });
    expect({ widgets, stacks }).toEqual(snapshot);
  });

  it("creates bounded stacks and prevents a fifth visible slot", () => {
    const widgets = ["one", "two", "three", "four"].map((id) => ({
      id,
      packageID: clock.packageID,
      digest: clock.digest,
      enabled: false,
      grants: [],
    }));
    const onChange = vi.fn();
    const { rerender } = render(
      <WidgetSettings
        instances={widgets}
        stacks={[]}
        catalog={[clock, network]}
        onChange={onChange}
        language="en"
      />,
    );
    expect(screen.getByRole("button", { name: "Add widget" })).toBeDisabled();
    fireEvent.click(screen.getByLabelText("Stack one"));
    fireEvent.click(screen.getByLabelText("Stack two"));
    fireEvent.click(screen.getByRole("button", { name: "Create stack" }));
    const changed = onChange.mock.calls.at(-1)?.[0];
    expect(changed.stacks[0]).toEqual(
      expect.objectContaining({ members: ["one", "two"], activeID: "one" }),
    );
    rerender(
      <WidgetSettings
        instances={changed.widgets}
        stacks={changed.stacks}
        catalog={[clock, network]}
        onChange={onChange}
        language="en"
      />,
    );
    expect(screen.getByRole("button", { name: "Add widget" })).toBeEnabled();
  });

  it("edits stack membership without leaving an invalid active member", () => {
    const widgets = ["one", "two", "three"].map((id) => ({
      id,
      packageID: clock.packageID,
      digest: clock.digest,
      enabled: false,
      grants: [],
    }));
    const stacks = [
      { id: "group", name: "Status", members: ["one", "two", "three"], activeID: "three" },
    ];
    const onChange = renderEditor(widgets, stacks);
    fireEvent.click(screen.getByLabelText("Status · three"));
    expect(onChange).toHaveBeenCalledWith({
      widgets,
      stacks: [{ ...stacks[0], members: ["one", "two"], activeID: "one" }],
    });
  });

  it("uses localized catalog and capability copy", () => {
    render(
      <WidgetSettings
        instances={[
          {
            id: "clock",
            packageID: clock.packageID,
            digest: clock.digest,
            enabled: false,
            grants: [],
          },
        ]}
        stacks={[]}
        catalog={[clock]}
        onChange={() => {}}
        language="pt-BR"
      />,
    );
    expect(screen.getByText("Relógio")).toBeVisible();
    expect(screen.getByText("Obrigatório · Ler a hora local")).toBeVisible();
  });

  it("rejects an out-of-range numeric setting before emitting", () => {
    const measured: WidgetCatalogDescriptor = {
      ...network,
      settings: [
        {
          id: "interval",
          type: "number",
          name: { en: "Interval" },
          defaultNumber: 5,
          min: 1,
          max: 10,
        },
      ],
    };
    const instance = {
      id: "network",
      packageID: measured.packageID,
      digest: measured.digest,
      enabled: false,
      grants: [],
    };
    const onChange = vi.fn();
    render(
      <WidgetSettings
        instances={[instance]}
        stacks={[]}
        catalog={[measured]}
        onChange={onChange}
      />,
    );
    fireEvent.change(screen.getByLabelText("Interval"), { target: { value: "11" } });
    expect(screen.getByRole("alert")).toHaveTextContent("Enter a valid value.");
    expect(onChange).not.toHaveBeenCalled();
  });
});

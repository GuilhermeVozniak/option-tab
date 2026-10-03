import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { type Lang, makeT } from "../lib/i18n";
import type { InstanceState, WidgetLease } from "../lib/widget-types";
import { WidgetView } from "./WidgetView";

const lease: WidgetLease = {
  controllerEpoch: 1,
  displayUUID: "screen",
  session: 1,
  profileID: "profile",
  instanceID: "audio",
  digest: "owned",
  admissionEpoch: 1,
  revision: 1,
};
const readings = [
  ["battery.charging", "Charging", "Carregando", "Cargando"],
  ["battery.notCharging", "Not charging", "Não está carregando", "No está cargando"],
  ["battery.power", "Battery power", "Alimentação por bateria", "Alimentación por batería"],
  ["battery.external", "External power", "Alimentação externa", "Alimentación externa"],
  [
    "battery.unknown",
    "Unknown power source",
    "Fonte de alimentação desconhecida",
    "Fuente de alimentación desconocida",
  ],
  ["network.connected", "Connected", "Conectado", "Conectado"],
  ["network.disconnected", "Disconnected", "Desconectado", "Desconectado"],
  ["network.none", "No connection", "Sem conexão", "Sin conexión"],
  ["network.wifi", "Wi-Fi", "Wi-Fi", "Wi-Fi"],
  ["network.ethernet", "Ethernet", "Ethernet", "Ethernet"],
  ["network.vpn", "VPN", "VPN", "VPN"],
  ["network.other", "Other network", "Outra rede", "Otra red"],
  ["network.unknown", "Unknown network", "Rede desconhecida", "Red desconocida"],
  ["audio.muted", "Muted", "Silenciado", "Silenciado"],
  ["audio.notMuted", "Not muted", "Não silenciado", "No silenciado"],
] as const;
const actions = () => ({
  options: vi.fn().mockResolvedValue({ options: [] }),
  perform: vi.fn().mockResolvedValue(undefined),
  asset: vi.fn().mockResolvedValue(""),
});
const state = (root: InstanceState["root"]): InstanceState => ({ lease, status: "ready", root });
const outputButton = {
  key: "root",
  kind: "button",
  status: "ready",
  text: "Choose output",
  textKey: "audio.chooseOutput",
  actionToken: "action",
} as const;

describe("widget product localization", () => {
  for (const [index, language] of (["en", "pt-BR", "es"] as Lang[]).entries()) {
    it(`renders trusted readings in ${language} while preserving literal values`, () => {
      const root = {
        key: "root",
        kind: "column",
        status: "ready",
        children: readings.slice(0, 8).map(([key]) => ({
          key,
          kind: "text",
          status: "ready",
          text: `raw:${key}`,
          textKey: key,
        })),
      };
      // Keep each validated layout below the sixteen-child limit.
      const tree: InstanceState["root"] = {
        key: "root",
        kind: "column",
        status: "ready",
        children: [
          {
            ...root,
            key: "group-a",
            kind: "column",
            children: root.children.map((node) => ({ ...node, kind: "text" })),
          },
          {
            key: "group-b",
            kind: "column",
            status: "ready",
            children: readings.slice(8).map(([key]) => ({
              key,
              kind: "text",
              status: "ready",
              text: `raw:${key}`,
              textKey: key,
            })),
          },
          { key: "device", kind: "text", status: "ready", text: "Connected" },
          {
            key: "community",
            kind: "button",
            status: "ready",
            text: "Choose output",
            actionToken: "community",
          },
        ],
      };
      const { container } = render(
        <WidgetView state={state(tree)} actions={actions()} t={makeT(language)} />,
      );
      expect(
        [...container.querySelectorAll(".ot-widget-text")].map((node) => node.textContent),
      ).toEqual([...readings.map((row) => row[index + 1]), "Connected"]);
      expect(screen.getByRole("button", { name: "Choose output" })).toBeVisible();
    });
  }
  it("changes an admitted chooser language without replacing authority or translating device names", async () => {
    const api = actions();
    api.options.mockResolvedValue({
      options: [
        { token: "unnamed", label: "Audio output", labelKey: "audio.output" },
        { token: "named", label: "Audio output" },
      ],
    });
    const { rerender } = render(
      <WidgetView state={state(outputButton)} actions={api} t={makeT("pt-BR")} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Escolher saída" }));
    await screen.findByRole("button", { name: "Saída de áudio" });
    expect(screen.getByRole("button", { name: "Audio output" })).toBeVisible();
    rerender(<WidgetView state={state(outputButton)} actions={api} t={makeT("en")} />);
    expect(screen.getByRole("button", { name: "Choose output" })).toBeVisible();
    expect(screen.getAllByRole("button", { name: "Audio output" })).toHaveLength(2);
    rerender(<WidgetView state={state(outputButton)} actions={api} t={makeT("es")} />);
    expect(screen.getByRole("button", { name: "Elegir salida" })).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Salida de audio" }));
    await waitFor(() => expect(api.perform).toHaveBeenCalledWith(lease, "action", "unnamed", null));
    expect(api.options).toHaveBeenCalledTimes(1);
  });
  for (const [reason, key, pt, es] of [
    [
      new Error("widgets: action busy"),
      "Widget action in progress",
      "Ação do widget em andamento",
      "Acción del widget en curso",
    ],
    [
      new Error("widgets: retired lease"),
      "This widget action is no longer available. Open it again.",
      "Esta ação do widget não está mais disponível. Abra-a novamente.",
      "Esta acción del widget ya no está disponible. Ábrela de nuevo.",
    ],
    [
      new Error("widgets: unavailable"),
      "Action unavailable",
      "Ação indisponível",
      "Acción no disponible",
    ],
    [
      new Error("private device detail"),
      "Action unavailable",
      "Ação indisponível",
      "Acción no disponible",
    ],
  ] as const) {
    it(`presents ${key} without raw backend errors and updates the visible language`, async () => {
      const api = actions();
      api.options.mockRejectedValue(reason);
      const { rerender } = render(
        <WidgetView state={state(outputButton)} actions={api} t={makeT("pt-BR")} />,
      );
      fireEvent.click(screen.getByRole("button"));
      expect(await screen.findByRole("alert")).toHaveTextContent(pt);
      rerender(<WidgetView state={state(outputButton)} actions={api} t={makeT("es")} />);
      expect(screen.getByRole("alert")).toHaveTextContent(es);
      expect(screen.getByRole("alert")).not.toHaveTextContent(reason.message);
    });
  }
});

it("keeps translated readings visible when optional fields and controls omit unavailable payloads", () => {
  const root = {
    key: "root",
    kind: "column",
    status: "partial",
    children: [
      {
        key: "connected",
        kind: "text",
        status: "ready",
        text: "true",
        textKey: "network.connected",
      },
      { key: "optional-reading", kind: "text", status: "unavailable" },
      { key: "optional-history", kind: "sparkline", status: "unavailable" },
      {
        key: "optional-action",
        kind: "button",
        status: "unavailable",
        text: "Choose output",
        textKey: "audio.chooseOutput",
      },
    ],
  } as const;
  render(
    <WidgetView
      state={{ lease, status: "partial", root: JSON.parse(JSON.stringify(root)) }}
      actions={actions()}
      t={makeT("es")}
    />,
  );
  expect(screen.getByText("Conectado")).toBeVisible();
  expect(screen.getByRole("button", { name: "Elegir salida" })).toBeDisabled();
  expect(screen.queryByRole("alert")).toBeNull();
});

it("still rejects a ready action without authority and bounds unavailable payloads", () => {
  const api = actions();
  const { rerender } = render(
    <WidgetView
      state={state({ key: "root", kind: "button", status: "ready", text: "Cannot run" })}
      actions={api}
    />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent("Widget content unavailable");
  expect(screen.queryByRole("button")).toBeNull();
  rerender(
    <WidgetView
      state={state({ key: "root", kind: "text", status: "unavailable", text: "x".repeat(4097) })}
      actions={api}
    />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent("Widget content unavailable");
  expect(api.perform).not.toHaveBeenCalled();
});

it("accepts an empty ready reading omitted by the host JSON while preserving sibling values", () => {
  // RenderNode.text uses omitempty: an admitted empty device name has no text
  // property on the wire, even though the reading itself is ready.
  const root: InstanceState["root"] = {
    key: "root",
    kind: "column",
    status: "ready",
    children: [
      { key: "output-name", kind: "text", status: "ready" },
      {
        key: "muted",
        kind: "text",
        status: "ready",
        text: "false",
        textKey: "audio.notMuted",
      },
    ],
  };
  const { container } = render(
    <WidgetView state={state(root)} actions={actions()} t={makeT("es")} />,
  );
  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.getByText("No silenciado")).toBeVisible();
  expect(container.querySelector(".ot-widget-text")).toBeEmptyDOMElement();
});

it("localizes an execution refusal while preserving the admitted chooser", async () => {
  const api = actions();
  api.perform.mockRejectedValue(new Error("widgets: action busy"));
  render(<WidgetView state={state(outputButton)} actions={api} t={makeT("es")} />);
  fireEvent.click(screen.getByRole("button", { name: "Elegir salida" }));
  fireEvent.click(await screen.findByRole("button", { name: "Ejecutar acción" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Acción del widget en curso");
  expect(screen.getByRole("button", { name: "Ejecutar acción" })).toBeEnabled();
  expect(api.perform).toHaveBeenCalledWith(lease, "action", "", null);
});

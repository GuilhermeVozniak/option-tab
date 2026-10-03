import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { makeT } from "../lib/i18n";
import { DockMonitorLock } from "./DockMonitorLock";

const value = {
  enabled: true,
  target: "display" as const,
  displayUUID: "gone-uuid",
  bypassModifier: "option" as const,
};
const props = {
  value,
  displays: [],
  t: (s: string) => s,
  onChange: vi.fn(),
  onEnable: vi.fn(),
  onPlace: vi.fn(),
  onCancel: vi.fn(),
};

it("translates runtime reasons and unknown failures using the current language", () => {
  const state = {
    session: 1,
    revision: 1,
    generation: 1,
    sequence: 1,
    observedAtMs: 0,
    status: "disconnected",
    reason: "selected display is disconnected",
    targetUUID: "gone-uuid",
    actualUUID: "",
    edge: "bottom",
    displays: [],
    placementAvailable: false,
  };
  const view = render(
    <DockMonitorLock
      {...props}
      state={state}
      error="native allocation failed"
      t={makeT("pt-BR")}
    />,
  );
  expect(screen.getByText(/^Estado:/)).toHaveTextContent("Monitor desconectado");
  expect(screen.getByRole("alert")).toHaveTextContent(
    "A proteção do Dock no monitor está indisponível. Tente novamente.",
  );
  view.rerender(
    <DockMonitorLock {...props} state={state} error="native allocation failed" t={makeT("es")} />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent(
    "La protección del Dock en la pantalla no está disponible. Inténtalo de nuevo.",
  );
  expect(
    screen.queryByText(/native allocation failed|selected display is disconnected/),
  ).toBeNull();
});

it.each(["pt-BR", "es"] as const)("localizes monitor controls in %s", (language) => {
  const t = makeT(language);
  render(<DockMonitorLock {...props} t={t} />);
  expect(screen.getByRole("checkbox", { name: t("Lock Dock to a monitor") })).toBeChecked();
  expect(screen.getByRole("combobox", { name: t("Target monitor") })).toHaveValue("gone-uuid");
  expect(screen.getByRole("combobox", { name: t("Bypass modifier") })).toHaveValue("option");
});

it.each([
  "pt-BR",
  "es",
] as const)("preserves the Accessibility permission cause in %s", (language) => {
  const t = makeT(language);
  render(<DockMonitorLock {...props} error="Accessibility permission unavailable" t={t} />);
  expect(screen.getByRole("alert")).toHaveTextContent(
    t("Accessibility permission is required for this action."),
  );
});

it("preserves a disconnected UUID and requests accessibility only on explicit enable", () => {
  const onChange = vi.fn(),
    onEnable = vi.fn();
  render(
    <DockMonitorLock
      {...props}
      value={{ ...value, enabled: false }}
      displays={[
        {
          uuid: "main-uuid",
          id: 1,
          name: "Studio Display",
          bounds: { x: 0, y: 0, w: 100, h: 100 },
          scale: 2,
          main: true,
          mirrored: false,
        },
      ]}
      onChange={onChange}
      onEnable={onEnable}
    />,
  );
  expect(screen.getByRole("option", { name: /Disconnected display/ })).toHaveValue("gone-uuid");
  expect(screen.getByRole("option", { name: "Studio Display" })).toHaveValue("main-uuid");
  fireEvent.click(screen.getByLabelText("Lock Dock to a monitor"));
  expect(onChange).toHaveBeenCalledWith({ ...value, enabled: true });
  expect(onEnable).toHaveBeenCalledOnce();
});

it("offers cancellation only after runtime reports accepted placement and surfaces errors", () => {
  const { rerender } = render(
    <DockMonitorLock
      {...props}
      error="native refused"
      state={{
        session: 1,
        revision: 1,
        generation: 1,
        sequence: 1,
        observedAtMs: 0,
        status: "starting",
        reason: "",
        targetUUID: "",
        actualUUID: "",
        edge: "",
        displays: [],
        placementAvailable: true,
      }}
    />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Dock monitor protection is unavailable. Try again.",
  );
  expect(screen.getByRole("button", { name: "Move Dock here" })).toBeDisabled();
  expect(screen.queryByRole("button", { name: "Cancel placement" })).toBeNull();
  rerender(
    <DockMonitorLock
      {...props}
      state={{
        session: 3,
        revision: 4,
        generation: 5,
        sequence: 2,
        observedAtMs: 0,
        status: "protected",
        reason: "",
        targetUUID: "gone-uuid",
        actualUUID: "gone-uuid",
        edge: "bottom",
        displays: [],
        placementAvailable: true,
      }}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Move Dock here" }));
  expect(props.onPlace).toHaveBeenCalledWith(3, 4, 5);
  rerender(
    <DockMonitorLock
      {...props}
      state={{
        session: 1,
        revision: 1,
        generation: 1,
        sequence: 2,
        observedAtMs: 0,
        status: "placing",
        reason: "",
        targetUUID: "",
        actualUUID: "",
        edge: "",
        displays: [],
        placementAvailable: true,
      }}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Cancel placement" }));
  expect(props.onCancel).toHaveBeenCalled();
});

it("shows honest manual guidance when native placement is unavailable", () => {
  render(
    <DockMonitorLock
      {...props}
      state={{
        session: 1,
        revision: 1,
        generation: 1,
        sequence: 1,
        observedAtMs: 0,
        status: "awaitingPlacement",
        reason: "",
        targetUUID: "gone-uuid",
        actualUUID: "",
        edge: "bottom",
        displays: [],
        placementAvailable: false,
      }}
    />,
  );
  expect(screen.queryByRole("button", { name: "Move Dock here" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Cancel placement" })).toBeNull();
  expect(screen.getByText(/Move the Dock to the selected display manually/)).toBeVisible();
});

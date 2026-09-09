import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
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
  expect(screen.getByRole("alert")).toHaveTextContent("native refused");
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

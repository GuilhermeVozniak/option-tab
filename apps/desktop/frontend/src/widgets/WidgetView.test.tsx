import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ActionOptions, InstanceState, Lease } from "../lib/widget-types";
import { WidgetView } from "./WidgetView";

const lease: Lease = {
  controllerEpoch: 2,
  displayUUID: "display-a",
  session: 3,
  profileID: "work",
  instanceID: "audio",
  digest: "sha256:owned",
  admissionEpoch: 4,
  revision: 5,
};
const state = (root: InstanceState["root"], patch: Partial<InstanceState> = {}): InstanceState => ({
  lease,
  status: "ready",
  reason: "",
  root,
  ...patch,
});
const actions = () => ({
  options: vi.fn<(_: Lease, __: string) => Promise<ActionOptions>>().mockResolvedValue({
    options: [],
  }),
  perform: vi.fn().mockResolvedValue(undefined),
  asset: vi.fn().mockResolvedValue("data:image/png;base64,aWNvbg=="),
});

describe("WidgetView", () => {
  it("renders only the fixed trusted node vocabulary and escapes hostile text", () => {
    const api = actions();
    const { container } = render(
      <WidgetView
        state={state({
          key: "root",
          kind: "column",
          status: "ready",
          children: [
            { key: "text", kind: "text", status: "ready", text: '<img src=x onerror="x">' },
            { key: "progress", kind: "progress", status: "ready", progress: 0.45 },
            { key: "line", kind: "sparkline", status: "ready", history: [0, 0.5, 1] },
          ],
        })}
        actions={api}
      />,
    );
    expect(screen.getByText('<img src=x onerror="x">')).toBeVisible();
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "45");
    expect(container.querySelector("svg polyline")).toBeTruthy();
  });

  it("degrades a malformed or over-deep tree without rendering its payload", () => {
    let root: InstanceState["root"] = { key: "leaf", kind: "text", status: "ready", text: "deep" };
    for (let depth = 0; depth < 9; depth++)
      root = { key: `depth-${depth}`, kind: "row", status: "ready", children: [root] };
    render(<WidgetView state={state(root)} actions={actions()} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Widget content unavailable");
    expect(screen.queryByText("deep")).toBeNull();
  });

  it("shows readable instance and node status without exposing backend reason text as markup", () => {
    render(
      <WidgetView
        state={state(
          { key: "root", kind: "text", status: "unavailable", text: "" },
          { status: "grantRequired", reason: "<b>permission</b>" },
        )}
        actions={actions()}
      />,
    );
    expect(screen.getByRole("status")).toHaveTextContent("Permission required");
    expect(screen.getByText("<b>permission</b>")).toBeVisible();
    expect(document.querySelector("b")).toBeNull();
  });

  it("keeps available children visible when optional data makes the instance partial", () => {
    render(
      <WidgetView
        state={state(
          {
            key: "root",
            kind: "row",
            status: "partial",
            children: [
              { key: "available", kind: "text", status: "ready", text: "Connected" },
              { key: "optional", kind: "text", status: "unavailable", text: "" },
            ],
          },
          { status: "partial" },
        )}
        actions={actions()}
      />,
    );
    expect(screen.getByText("Connected")).toBeVisible();
    expect(screen.getByText("Widget unavailable")).toBeVisible();
  });

  it("loads only PNG assets and discards a late result after the lease changes", async () => {
    let resolveAsset!: (value: string) => void;
    const api = actions();
    api.asset.mockReturnValueOnce(new Promise((resolve) => (resolveAsset = resolve)));
    const icon = { key: "icon", kind: "icon", status: "ready", assetToken: "asset-1" } as const;
    const { rerender } = render(<WidgetView state={state(icon)} actions={api} />);
    expect(api.asset).toHaveBeenCalledWith(lease, "asset-1");
    const nextLease = { ...lease, revision: 6 };
    rerender(<WidgetView state={{ ...state(icon), lease: nextLease }} actions={api} />);
    resolveAsset("data:image/png;base64,b2xk");
    await waitFor(() =>
      expect(screen.getByRole("img")).toHaveAttribute("src", "data:image/png;base64,aWNvbg=="),
    );
    expect(screen.getByRole("img")).not.toHaveAttribute("src", "data:image/png;base64,b2xk");
  });

  it("requires a second explicit choice before performing an opaque option action", async () => {
    const api = actions();
    api.options.mockResolvedValue({
      options: [
        { token: "opaque-a", label: "Studio Display" },
        { token: "opaque-b", label: "Headphones" },
      ],
    });
    render(
      <WidgetView
        state={state({
          key: "choose",
          kind: "button",
          status: "ready",
          text: "Choose sound output",
          actionToken: "action-7",
        })}
        actions={api}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Choose sound output" }));
    await screen.findByRole("button", { name: "Headphones" });
    expect(api.perform).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Headphones" }));
    expect(api.perform).toHaveBeenCalledWith(lease, "action-7", "opaque-b", null);
    await waitFor(() => expect(screen.queryByLabelText("Widget action")).toBeNull());
  });

  it("keeps an admitted chooser across value-only revisions and performs with its original lease", async () => {
    const api = actions();
    api.options.mockResolvedValue({
      options: [{ token: "device-current", label: "Desk speakers" }],
    });
    const button = {
      key: "output",
      kind: "button",
      status: "ready",
      text: "Choose sound output",
      actionToken: "stable-authority",
    } as const;
    const { rerender } = render(<WidgetView state={state(button)} actions={api} />);
    fireEvent.click(screen.getByRole("button", { name: "Choose sound output" }));
    await screen.findByRole("button", { name: "Desk speakers" });

    rerender(
      <WidgetView
        state={{ ...state(button), lease: { ...lease, revision: lease.revision + 1 } }}
        actions={api}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Desk speakers" }));
    await waitFor(() =>
      expect(api.perform).toHaveBeenCalledWith(lease, "stable-authority", "device-current", null),
    );
    await waitFor(() => expect(screen.queryByLabelText("Widget action")).toBeNull());
  });

  it("retires an admitted chooser when its action authority changes", async () => {
    const api = actions();
    api.options.mockResolvedValue({
      options: [{ token: "old-option", label: "Old output" }],
    });
    const button = {
      key: "output",
      kind: "button",
      status: "ready",
      text: "Choose sound output",
      actionToken: "authority-a",
    } as const;
    const { rerender } = render(<WidgetView state={state(button)} actions={api} />);
    fireEvent.click(screen.getByRole("button", { name: "Choose sound output" }));
    await screen.findByRole("button", { name: "Old output" });

    rerender(
      <WidgetView
        state={{
          ...state(button),
          lease: { ...lease, revision: lease.revision + 1 },
          root: { ...button, actionToken: "authority-b" },
        }}
        actions={api}
      />,
    );

    expect(screen.queryByRole("button", { name: "Old output" })).toBeNull();
  });

  it("submits a bounded range explicitly and disables commands while busy", async () => {
    let finish!: () => void;
    const api = actions();
    api.options.mockResolvedValue({ options: [], range: { min: 0, max: 120, step: 5 } });
    api.perform.mockReturnValue(new Promise<void>((resolve) => (finish = resolve)));
    render(
      <WidgetView
        state={state({
          key: "seek",
          kind: "button",
          status: "ready",
          text: "Seek",
          actionToken: "seek-token",
        })}
        actions={api}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Seek" }));
    const slider = await screen.findByRole("slider");
    fireEvent.change(slider, { target: { value: "35" } });
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(api.perform).toHaveBeenCalledWith(lease, "seek-token", "", 35);
    expect(screen.getByRole("button", { name: "Apply" })).toBeDisabled();
    finish();
    await waitFor(() => expect(screen.queryByRole("button", { name: "Apply" })).toBeNull());
  });

  it("drops late option and action failures after an exact lease replacement", async () => {
    let rejectOptions!: (reason: Error) => void;
    let rejectAction!: (reason: Error) => void;
    const api = actions();
    api.options.mockReturnValueOnce(new Promise((_, reject) => (rejectOptions = reject)));
    const button = {
      key: "action",
      kind: "button",
      status: "ready",
      text: "Action",
      actionToken: "token",
    } as const;
    const { rerender } = render(<WidgetView state={state(button)} actions={api} />);
    fireEvent.click(screen.getByRole("button", { name: "Action" }));
    rerender(
      <WidgetView state={{ ...state(button), lease: { ...lease, session: 8 } }} actions={api} />,
    );
    rejectOptions(new Error("old options"));
    await Promise.resolve();
    expect(screen.queryByText("old options")).toBeNull();

    api.options.mockResolvedValueOnce({ options: [] });
    api.perform.mockReturnValueOnce(new Promise((_, reject) => (rejectAction = reject)));
    fireEvent.click(screen.getByRole("button", { name: "Action" }));
    fireEvent.click(await screen.findByRole("button", { name: "Run action" }));
    rerender(
      <WidgetView state={{ ...state(button), lease: { ...lease, session: 9 } }} actions={api} />,
    );
    rejectAction(new Error("old action"));
    await Promise.resolve();
    expect(screen.queryByText("old action")).toBeNull();
  });
});

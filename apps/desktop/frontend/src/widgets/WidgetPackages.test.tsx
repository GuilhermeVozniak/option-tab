import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { makeT } from "../lib/i18n";
import type { WidgetCatalogDescriptor, WidgetPackageReview } from "../lib/widget-types";
import { WidgetPackages } from "./WidgetPackages";

const pkg: WidgetCatalogDescriptor = {
  packageID: "org.example.status",
  digest: "d".repeat(64),
  version: "1.0.0",
  name: { en: "Status card" },
  description: { en: "Shows local status" },
  requiredCapabilities: ["network.status.read"],
  optionalCapabilities: ["audio.output.select"],
  settings: [],
  builtin: false,
};
const review: WidgetPackageReview = {
  token: "opaque-review",
  sourceName: "status.otwidget",
  package: pkg,
  expiresAt: "2026-09-07T15:00:00Z",
  alreadyInstalled: false,
};
const actions = () => ({
  review: vi.fn(async () => review),
  install: vi.fn(async () => pkg),
  cancel: vi.fn(async () => {}),
  remove: vi.fn(async () => {}),
});

describe("WidgetPackages", () => {
  it.each([
    ["invalidFile", "This widget package file is invalid."],
    ["invalidPackage", "This widget package is invalid."],
    ["incompatibleVersion", "This widget package requires a newer version of Option Tab."],
    ["settingsSaveFailed", "Widget settings could not be saved. Try again."],
    ["busy", "Another package operation is in progress. Try again."],
    ["unavailable", "Local package management is unavailable."],
    ["retired", "This package operation is no longer available. Try again."],
    ["reviewExpired", "This package review expired. Choose the file again."],
  ])("presents the stable package error %s without internal codes", async (code, expected) => {
    const api = actions();
    api.review.mockRejectedValueOnce(new Error(`widget package: ${code}`));
    render(
      <WidgetPackages
        catalog={[]}
        status={{ available: true, busy: false, reason: "" }}
        actions={api}
        onRefresh={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(expected);
  });

  it("translates a pending failure and existing feedback in the current language", async () => {
    let fail!: (reason: Error) => void;
    const api = actions();
    api.review.mockReturnValueOnce(
      new Promise((_, reject) => {
        fail = reject;
      }),
    );
    const props = {
      catalog: [],
      status: { available: true, busy: false, reason: "catalogInvalid" },
      actions: api,
      onRefresh: () => {},
    };
    const { rerender } = render(<WidgetPackages {...props} t={makeT("en")} />);
    fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
    rerender(<WidgetPackages {...props} t={makeT("es")} />);
    await act(async () => fail(new Error("widget package: invalidPackage")));
    expect(screen.getByRole("alert")).toHaveTextContent("Este paquete de widget no es válido.");
    expect(screen.getByRole("status")).toHaveTextContent(
      "No se pudieron cargar algunos paquetes de widgets instalados.",
    );
    rerender(<WidgetPackages {...props} t={makeT("pt-BR")} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Este pacote de widget é inválido.");
    expect(screen.getByRole("status")).toHaveTextContent(
      "Não foi possível carregar alguns pacotes de widgets instalados.",
    );
  });

  it.each([
    "widget package: inventedCode /private/secret",
    "Error: provider failure",
    "constructor",
    "__proto__",
  ])("does not expose unknown package details: %s", async (message) => {
    const api = actions();
    api.review.mockRejectedValueOnce(new Error(message));
    render(
      <WidgetPackages
        catalog={[]}
        status={{ available: true, busy: false, reason: message }}
        actions={api}
        onRefresh={() => {}}
        t={makeT("es")}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Revisar paquete local…" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "No se pudo completar la operación del paquete de widget. Vuelve a intentarlo.",
    );
    expect(screen.getByRole("status")).toHaveTextContent(
      "No se pudo completar la operación del paquete de widget. Vuelve a intentarlo.",
    );
    expect(screen.getByRole("alert")).not.toHaveTextContent(message);
  });

  it("localizes a package removal status without rewriting community metadata", () => {
    render(
      <WidgetPackages
        catalog={[pkg]}
        status={{ available: true, busy: false, reason: "removeFailed" }}
        actions={actions()}
        onRefresh={() => {}}
        t={makeT("pt-BR")}
        language="pt-BR"
      />,
    );
    expect(screen.getByRole("status")).toHaveTextContent(
      "Não foi possível remover o pacote de widget. Tente novamente.",
    );
    expect(screen.getByText("Status card · 1.0.0")).toBeVisible();
  });

  it("keeps cancellation feedback neutral", async () => {
    const api = actions();
    api.review.mockReturnValueOnce(new Promise(() => {}));
    api.cancel.mockRejectedValueOnce(new Error("context canceled"));
    render(
      <WidgetPackages
        catalog={[]}
        status={{ available: true, busy: false, reason: "" }}
        actions={api}
        onRefresh={() => {}}
        t={makeT("es")}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Revisar paquete local…" }));
    await act(async () =>
      fireEvent.click(screen.getByRole("button", { name: "Cancelar revisión" })),
    );
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByRole("button", { name: "Revisar paquete local…" })).toBeEnabled();
  });

  it("shows generic feedback when an operation fails without an error message", async () => {
    const api = actions();
    api.review.mockRejectedValueOnce(new Error(""));
    render(
      <WidgetPackages
        catalog={[]}
        status={{ available: true, busy: false, reason: "" }}
        actions={api}
        onRefresh={() => {}}
      />,
    );
    expect(screen.queryByRole("status")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "The widget package operation could not be completed. Try again.",
    );
  });

  it.each([
    "pending",
    "reviewed",
  ] as const)("ignores a stale %s cancellation failure after a new review or disable", async (stage) => {
    for (const successor of ["new review", "disabled"] as const) {
      let rejectCancel!: (reason: Error) => void;
      const api = actions();
      if (stage === "pending") api.review.mockReturnValueOnce(new Promise(() => {}));
      api.cancel.mockReturnValueOnce(
        new Promise((_, reject) => {
          rejectCancel = reject;
        }),
      );
      const props = {
        catalog: [],
        status: { available: true, busy: false, reason: "" },
        actions: api,
        onRefresh: () => {},
      };
      const view = render(<WidgetPackages {...props} />);
      fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
      if (stage === "reviewed") await screen.findByText("status.otwidget");
      fireEvent.click(
        screen.getByRole("button", {
          name: stage === "pending" ? "Cancel review" : "Close review",
        }),
      );
      expect(api.cancel).toHaveBeenCalledWith(stage === "pending" ? "" : "opaque-review");
      if (successor === "new review") {
        api.review.mockRejectedValueOnce(new Error("widget package: invalidFile"));
        fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
        expect(await screen.findByRole("alert")).toHaveTextContent(
          "This widget package file is invalid.",
        );
      } else {
        view.rerender(
          <WidgetPackages {...props} status={{ available: false, busy: false, reason: "" }} />,
        );
      }
      await act(async () => rejectCancel(new Error("widget package: busy")));
      if (successor === "new review")
        expect(screen.getByRole("alert")).toHaveTextContent("This widget package file is invalid.");
      else expect(screen.queryByRole("alert")).toBeNull();
      view.unmount();
    }
  });

  it.each([
    "pending",
    "reviewed",
  ] as const)("keeps current %s cancellation failures visible", async (stage) => {
    const api = actions();
    if (stage === "pending") api.review.mockReturnValueOnce(new Promise(() => {}));
    api.cancel.mockRejectedValueOnce(new Error("widget package: busy"));
    render(
      <WidgetPackages
        catalog={[]}
        status={{ available: true, busy: false, reason: "" }}
        actions={api}
        onRefresh={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
    if (stage === "reviewed") await screen.findByText("status.otwidget");
    fireEvent.click(
      screen.getByRole("button", { name: stage === "pending" ? "Cancel review" : "Close review" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Another package operation is in progress. Try again.",
    );
  });

  it("keeps the installation owner until it finishes instead of allowing the consumed review to close", async () => {
    let finish!: (value: WidgetCatalogDescriptor) => void;
    const api = actions();
    api.install.mockReturnValue(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    const refresh = vi.fn();
    render(
      <WidgetPackages
        catalog={[]}
        status={{ available: true, busy: false, reason: "" }}
        actions={api}
        onRefresh={refresh}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
    await screen.findByText("status.otwidget");
    fireEvent.click(screen.getByRole("button", { name: "Install reviewed package" }));
    expect(screen.getByRole("button", { name: "Close review" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Close review" }));
    await act(async () => finish(pkg));
    expect(screen.queryByText("status.otwidget")).toBeNull();
    expect(screen.getByRole("button", { name: "Review local package…" })).toBeEnabled();
    expect(refresh).toHaveBeenCalledOnce();
    expect(api.cancel).not.toHaveBeenCalled();
  });

  it("prevents a new review from interrupting a pending removal", async () => {
    let finish!: () => void;
    const api = actions();
    api.remove.mockReturnValue(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    const refresh = vi.fn();
    render(
      <WidgetPackages
        catalog={[pkg]}
        status={{ available: true, busy: false, reason: "" }}
        actions={api}
        onRefresh={refresh}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Remove package" }));
    expect(screen.getByRole("button", { name: "Review local package…" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
    await act(async () => finish());
    expect(refresh).toHaveBeenCalledOnce();
    expect(screen.getByRole("button", { name: "Review local package…" })).toBeEnabled();
    expect(api.review).not.toHaveBeenCalled();
  });

  it("prevents removing a package while its chooser is pending", async () => {
    let finish!: (value: WidgetPackageReview) => void;
    const api = actions();
    api.review.mockReturnValue(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    render(
      <WidgetPackages
        catalog={[pkg]}
        status={{ available: true, busy: false, reason: "" }}
        actions={api}
        onRefresh={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
    expect(screen.getByRole("button", { name: "Remove package" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Remove package" }));
    await act(async () => finish(review));
    expect(screen.getByText("status.otwidget")).toBeInTheDocument();
    expect(api.remove).not.toHaveBeenCalled();
  });
  it("reviews immutable package facts before a separate install with no grants", async () => {
    const api = actions();
    const refresh = vi.fn();
    render(
      <WidgetPackages
        catalog={[]}
        status={{ available: true, busy: false, reason: "" }}
        actions={api}
        onRefresh={refresh}
      />,
    );
    expect(api.review).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
    expect(await screen.findByText("status.otwidget")).toBeVisible();
    expect(screen.getByText("Not independently verified")).toBeVisible();
    expect(screen.getByText("Installing does not grant data access or actions.")).toBeVisible();
    expect(api.install).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Install reviewed package" }));
    await waitFor(() => expect(api.install).toHaveBeenCalledWith("opaque-review"));
    await waitFor(() => expect(refresh).toHaveBeenCalled());
  });

  it("cancels an outstanding chooser explicitly and discards its late review", async () => {
    let resolve!: (value: WidgetPackageReview) => void;
    const api = actions();
    api.review.mockReturnValue(new Promise((done) => (resolve = done)));
    render(
      <WidgetPackages
        catalog={[]}
        status={{ available: true, busy: false, reason: "" }}
        actions={api}
        onRefresh={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
    fireEvent.click(screen.getByRole("button", { name: "Cancel review" }));
    expect(api.cancel).toHaveBeenCalledWith("");
    resolve(review);
    await Promise.resolve();
    expect(screen.queryByText("status.otwidget")).toBeNull();
  });

  it("retires a reviewed token when package management becomes unavailable", async () => {
    const api = actions();
    const { rerender } = render(
      <WidgetPackages
        catalog={[]}
        status={{ available: true, busy: false, reason: "" }}
        actions={api}
        onRefresh={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
    await screen.findByText("status.otwidget");
    rerender(
      <WidgetPackages
        catalog={[]}
        status={{ available: false, busy: false, reason: "inactive" }}
        actions={api}
        onRefresh={() => {}}
      />,
    );
    await waitFor(() => expect(screen.queryByText("status.otwidget")).toBeNull());
    expect(screen.getByText("Local package management is unavailable.")).toBeVisible();
  });
});

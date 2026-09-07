import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
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

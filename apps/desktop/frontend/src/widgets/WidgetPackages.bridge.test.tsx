import { act, fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import * as AppService from "../../bindings/option-tab/app.js";
import { makeT } from "../lib/i18n";
import { launcher } from "../lib/launcher-bridge";
import { WidgetPackages } from "./WidgetPackages";

vi.mock("../../bindings/option-tab/app.js", () => ({
  ReviewLocalWidgetPackage: vi.fn(),
}));

it.each([
  "context canceled",
  "widget package: invalidPackage",
])("handles native package review result %s through the production bridge", async (message) => {
  vi.mocked(AppService.ReviewLocalWidgetPackage).mockRejectedValueOnce(new Error(message));
  render(
    <WidgetPackages
      catalog={[]}
      status={{ available: true, busy: false, reason: "" }}
      actions={{
        review: launcher.reviewPackage,
        install: launcher.installPackage,
        cancel: launcher.cancelPackageReview,
        remove: launcher.removePackage,
      }}
      onRefresh={() => {}}
    />,
  );
  await act(async () =>
    fireEvent.click(screen.getByRole("button", { name: "Review local package…" })),
  );
  if (message === "context canceled") expect(screen.queryByRole("alert")).toBeNull();
  else expect(screen.getByRole("alert")).toHaveTextContent("This widget package is invalid.");
  expect(screen.getByRole("button", { name: "Review local package…" })).toBeEnabled();
  expect(screen.queryByRole("button", { name: "Install reviewed package" })).toBeNull();
});

it.each([
  ["pt-BR", "Este pacote de widget é inválido."],
  ["es", "Este paquete de widget no es válido."],
] as const)("localizes native package rejection through the bridge in %s", async (language, expected) => {
  vi.mocked(AppService.ReviewLocalWidgetPackage).mockRejectedValueOnce(
    new Error("widget package: invalidPackage"),
  );
  render(
    <WidgetPackages
      catalog={[]}
      status={{ available: true, busy: false, reason: "" }}
      actions={{
        review: launcher.reviewPackage,
        install: launcher.installPackage,
        cancel: launcher.cancelPackageReview,
        remove: launcher.removePackage,
      }}
      onRefresh={() => {}}
      t={makeT(language)}
    />,
  );
  await act(async () =>
    fireEvent.click(screen.getByRole("button", { name: makeT(language)("Review local package…") })),
  );
  expect(screen.getByRole("alert")).toHaveTextContent(expected);
  expect(screen.getByRole("alert")).not.toHaveTextContent("invalidPackage");
});

import { act, fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import * as AppService from "../../bindings/option-tab/app.js";
import { makeT } from "../lib/i18n";
import { LauncherItems } from "./LauncherItems";

// Keep the component and its default production bridge intact. Only the native
// Wails service is replaced with the persisted settings/icon responses.
vi.mock("../../bindings/option-tab/app.js", () => ({
  GetLauncherItemStatus: async () => ({ available: true, busy: false, reason: "" }),
  GetLauncherItemSettings: async (profileID: string) => {
    if (profileID !== "work") throw new Error("launcher items: profileMissing");
    return {
      profileID: "work",
      revision: "persisted",
      iconIDs: ["e".repeat(64)],
      references: [],
      items: [
        {
          id: "guide",
          kind: "link",
          label: "Saved guide",
          url: "https://example.com",
          iconID: "e".repeat(64),
        },
      ],
    };
  },
  GetLauncherItemIcon: async (id: string) => {
    if (id !== "e".repeat(64)) throw new Error("launcher items: unknownIcon");
    return { id, dataURL: "data:image/png;base64,aWNvbg==" };
  },
  ChooseLauncherItemReference: vi.fn(),
  ChooseLauncherItemIcon: vi.fn(),
  SetLauncherItems: vi.fn(),
}));

it("restores a persisted custom icon through the production bridge when settings reopen", async () => {
  const t = makeT("en");
  const first = render(<LauncherItems profileID="work" t={t} />);
  expect(await screen.findByAltText("Custom icon")).toHaveAttribute(
    "src",
    "data:image/png;base64,aWNvbg==",
  );
  first.unmount();

  render(<LauncherItems profileID="work" t={t} />);
  expect(await screen.findByAltText("Custom icon")).toHaveAttribute(
    "src",
    "data:image/png;base64,aWNvbg==",
  );
  expect(screen.getByText("Saved guide")).toBeInTheDocument();
});

it.each([
  "folder",
  "icon",
] as const)("silently retains the draft when the native %s chooser is cancelled", async (kind) => {
  vi.mocked(AppService.ChooseLauncherItemReference)
    .mockReset()
    .mockRejectedValueOnce(new Error("context canceled"));
  vi.mocked(AppService.ChooseLauncherItemIcon)
    .mockReset()
    .mockRejectedValueOnce(new Error("context canceled"));
  render(<LauncherItems profileID="work" t={makeT("en")} />);
  await screen.findByText("Saved guide");
  fireEvent.click(screen.getByRole("button", { name: "Add spacer" }));
  if (kind === "icon") fireEvent.click(screen.getByRole("button", { name: "Remove custom icon" }));
  await act(async () =>
    fireEvent.click(
      screen.getByRole("button", { name: kind === "folder" ? "Add folder" : "Choose custom icon" }),
    ),
  );
  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.getAllByTestId("launcher-item")).toHaveLength(2);
  expect(screen.getByText("Saved guide")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Add folder" })).toBeEnabled();
});

it("still reports a native selection error and a cancelled save", async () => {
  vi.mocked(AppService.ChooseLauncherItemReference)
    .mockReset()
    .mockRejectedValueOnce(new Error("launcher item: accessRequired"));
  vi.mocked(AppService.SetLauncherItems).mockRejectedValueOnce(new Error("context canceled"));
  render(<LauncherItems profileID="work" t={makeT("en")} />);
  await screen.findByText("Saved guide");
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Add folder" })));
  expect(screen.getByRole("alert")).toHaveTextContent("launcher item: accessRequired");
  await act(async () =>
    fireEvent.click(screen.getByRole("button", { name: "Save launcher items" })),
  );
  expect(screen.getByRole("alert")).toHaveTextContent("context canceled");
  expect(screen.getByText("Saved guide")).toBeInTheDocument();
});

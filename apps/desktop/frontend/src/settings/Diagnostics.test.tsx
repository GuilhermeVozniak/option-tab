import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { makeT } from "../lib/i18n";
import { Diagnostics, type DiagnosticsClient } from "./Diagnostics";

const snapshot = (token: string, json: string, recording = false) => ({
  token,
  json,
  expiresAt: "2026-09-07T12:00:00Z",
  recording,
  dropped: 2,
});
const client = (): DiagnosticsClient => ({
  review: vi.fn().mockResolvedValue(snapshot("review-1", '{"schemaVersion":1}')),
  start: vi.fn().mockResolvedValue(undefined),
  stop: vi.fn().mockResolvedValue(undefined),
  clear: vi.fn().mockResolvedValue(undefined),
  save: vi.fn().mockResolvedValue({ status: "saved" }),
});

it("does nothing until explicit review and saves the exact immutable token", async () => {
  const api = client();
  render(<Diagnostics t={(value) => value} client={api} />);
  expect(api.review).not.toHaveBeenCalled();
  expect(api.start).not.toHaveBeenCalled();
  expect(api.save).not.toHaveBeenCalled();
  fireEvent.click(screen.getByText("Diagnostics"));
  fireEvent.click(screen.getByRole("button", { name: "Review diagnostics" }));
  expect(await screen.findByLabelText("Diagnostics report preview")).toHaveTextContent(
    '{"schemaVersion":1}',
  );
  (api.review as ReturnType<typeof vi.fn>).mockResolvedValue(
    snapshot("review-2", '{"schemaVersion":2}'),
  );
  expect(screen.getByLabelText("Diagnostics report preview")).toHaveTextContent(
    '{"schemaVersion":1}',
  );
  fireEvent.click(screen.getByRole("button", { name: "Save report…" }));
  await waitFor(() => expect(api.save).toHaveBeenCalledWith("review-1"));
  expect(await screen.findByRole("status")).toHaveTextContent("Report saved");
});

it("refreshes only after explicit recording controls and treats save cancellation neutrally", async () => {
  const api = client();
  (api.review as ReturnType<typeof vi.fn>)
    .mockResolvedValueOnce(snapshot("before", "before"))
    .mockResolvedValueOnce(snapshot("after", "after", true));
  (api.save as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ status: "cancelled" });
  render(<Diagnostics t={(value) => value} client={api} />);
  fireEvent.click(screen.getByText("Diagnostics"));
  fireEvent.click(screen.getByRole("button", { name: "Review diagnostics" }));
  await screen.findByText("before");
  fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  await screen.findByText("after");
  expect(api.start).toHaveBeenCalledTimes(1);
  expect(api.review).toHaveBeenCalledTimes(2);
  fireEvent.click(screen.getByRole("button", { name: "Save report…" }));
  await waitFor(() => expect(api.save).toHaveBeenCalledWith("after"));
  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.queryByRole("status")).toBeNull();
});

it("maps stable expiry and destination errors and renders localized controls", async () => {
  const api = client();
  (api.save as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error("destinationExists"));
  render(<Diagnostics t={makeT("pt-BR")} client={api} />);
  fireEvent.click(screen.getByText("Diagnóstico"));
  fireEvent.click(screen.getByRole("button", { name: "Revisar diagnóstico" }));
  await screen.findByLabelText("Prévia do relatório de diagnóstico");
  fireEvent.click(screen.getByRole("button", { name: "Salvar relatório…" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Esse nome de arquivo já existe. Escolha um novo nome.",
  );
});

it("keeps cancellation neutral and offers a fresh review after token expiry", async () => {
  const api = client();
  (api.save as ReturnType<typeof vi.fn>)
    .mockRejectedValueOnce(new Error("cancelled"))
    .mockRejectedValueOnce(new Error("reviewExpired"));
  render(<Diagnostics t={(value) => value} client={api} />);
  fireEvent.click(screen.getByText("Diagnostics"));
  fireEvent.click(screen.getByRole("button", { name: "Review diagnostics" }));
  await screen.findByLabelText("Diagnostics report preview");
  fireEvent.click(screen.getByRole("button", { name: "Save report…" }));
  await waitFor(() => expect(api.save).toHaveBeenCalledTimes(1));
  expect(screen.queryByRole("alert")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Save report…" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "This preview expired. Refresh it before saving.",
  );
  fireEvent.click(screen.getAllByRole("button", { name: "Refresh preview" }).at(-1)!);
  await waitFor(() => expect(api.review).toHaveBeenCalledTimes(2));
});

import {
  ClearDiagnostics,
  GetDiagnosticsReview,
  SaveDiagnosticsReport,
  StartDiagnosticsRecording,
  StopDiagnosticsRecording,
} from "../../bindings/option-tab/app.js";

export interface DiagnosticsReview {
  token: string;
  json: string;
  expiresAt: string;
  recording: boolean;
  dropped: number;
}
export interface DiagnosticsSaveResult {
  status: "saved" | "cancelled";
}
export const diagnostics = {
  review: () => GetDiagnosticsReview() as Promise<DiagnosticsReview>,
  start: () => StartDiagnosticsRecording(),
  stop: () => StopDiagnosticsRecording(),
  clear: () => ClearDiagnostics(),
  save: (token: string) => SaveDiagnosticsReport(token) as Promise<DiagnosticsSaveResult>,
};

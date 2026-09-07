import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  type DiagnosticsReview,
  type DiagnosticsSaveResult,
  diagnostics,
} from "../lib/diagnostics-bridge";
import type { Translate } from "../lib/i18n";
import { ACTIONS_ROW, HINT } from "./shared";

export interface DiagnosticsClient {
  review: () => Promise<DiagnosticsReview>;
  start: () => Promise<void>;
  stop: () => Promise<void>;
  clear: () => Promise<void>;
  save: (token: string) => Promise<DiagnosticsSaveResult>;
}

export function Diagnostics({
  t,
  client = diagnostics,
}: {
  t: Translate;
  client?: DiagnosticsClient;
}) {
  const [review, setReview] = useState<DiagnosticsReview | null>(null);
  const [pending, setPending] = useState("");
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const operation = useRef(0);

  const loadReview = async (owner = ++operation.current) => {
    setPending("review");
    setError("");
    try {
      const next = await client.review();
      if (owner === operation.current) setReview(next);
    } catch (cause) {
      if (owner === operation.current)
        setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (owner === operation.current) setPending("");
    }
  };
  const mutate = async (kind: string, action: () => Promise<void>) => {
    const owner = ++operation.current;
    setPending(kind);
    setError("");
    setSaved(false);
    try {
      await action();
      if (owner !== operation.current) return;
      const next = await client.review();
      if (owner === operation.current) setReview(next);
    } catch (cause) {
      if (owner === operation.current)
        setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (owner === operation.current) setPending("");
    }
  };
  const save = async () => {
    if (!review) return;
    const owner = ++operation.current;
    const token = review.token;
    setPending("save");
    setError("");
    setSaved(false);
    try {
      const result = await client.save(token);
      if (owner === operation.current && result.status === "saved") setSaved(true);
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      if (owner === operation.current && !/cancelled/i.test(message)) setError(message);
    } finally {
      if (owner === operation.current) setPending("");
    }
  };
  const expired = /expired|token/i.test(error);
  const destinationExists = /destinationExists|destination exists/i.test(error);
  const busy = /(^|\W)busy(\W|$)/i.test(error);

  return (
    <details className="rounded-xl border border-border/70 bg-card/55 p-4">
      <summary className="cursor-pointer text-sm font-semibold">{t("Diagnostics")}</summary>
      <div className="mt-3 space-y-3">
        <p className={HINT}>
          {t(
            "Review a bounded local report with the app version, coarse statuses and diagnostic events. It excludes window, media and file content and is never uploaded automatically.",
          )}
        </p>
        {!review ? (
          <Button disabled={!!pending} onClick={() => void loadReview()}>
            {t(pending === "review" ? "Preparing preview…" : "Review diagnostics")}
          </Button>
        ) : (
          <>
            <div className={ACTIONS_ROW}>
              <Button variant="outline" disabled={!!pending} onClick={() => void loadReview()}>
                {t("Refresh preview")}
              </Button>
              {review.recording ? (
                <Button
                  variant="outline"
                  disabled={!!pending}
                  onClick={() => void mutate("stop", client.stop)}
                >
                  {t("Stop recording")}
                </Button>
              ) : (
                <Button
                  variant="outline"
                  disabled={!!pending}
                  onClick={() => void mutate("start", client.start)}
                >
                  {t("Start recording")}
                </Button>
              )}
              <Button
                variant="outline"
                disabled={!!pending}
                onClick={() => void mutate("clear", client.clear)}
              >
                {t("Clear diagnostics")}
              </Button>
              <Button disabled={!!pending} onClick={() => void save()}>
                {t("Save report…")}
              </Button>
            </div>
            <p className={HINT}>
              {review.recording
                ? t("Recording stops automatically after 10 minutes.")
                : t("Recording is off.")}{" "}
              {t("Dropped events")}: {review.dropped}
            </p>
            <pre
              aria-label={t("Diagnostics report preview")}
              className="max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-black/20 p-3 text-xs"
            >
              {review.json}
            </pre>
          </>
        )}
        {error ? (
          <div role="alert" className="text-sm text-red-600 dark:text-red-400">
            <span>
              {destinationExists
                ? t("That filename already exists. Choose a new name.")
                : expired
                  ? t("This preview expired. Refresh it before saving.")
                  : busy
                    ? t("Another diagnostics operation is already in progress.")
                    : error}
            </span>
            {expired ? (
              <Button variant="outline" onClick={() => void loadReview()}>
                {t("Refresh preview")}
              </Button>
            ) : null}
          </div>
        ) : null}
        {saved ? <p role="status">{t("Report saved")}</p> : null}
      </div>
    </details>
  );
}

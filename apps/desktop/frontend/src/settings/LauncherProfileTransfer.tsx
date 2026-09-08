import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import type { Translate } from "../lib/i18n";
import { jsonExportError } from "../lib/json-export-bridge";
import type {
  LauncherProfileImportReview,
  LauncherProfileTransferActions,
} from "../lib/launcher-profile-transfer-bridge";
import { ACTIONS_ROW, HINT } from "./shared";

const MAX_DOCUMENT_BYTES = 256 * 1024;

function readFileText(file: File): Promise<string> {
  if (typeof file.text === "function") return file.text();
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error("readFailed"));
    reader.onload = () => resolve(String(reader.result ?? ""));
    reader.readAsText(file);
  });
}

function friendlyError(cause: unknown, t: Translate): string {
  const message = cause instanceof Error ? cause.message : String(cause);
  if (/staleDigest/i.test(message))
    return t("The reviewed profile changed. Choose the file again.");
  if (/staleRevision/i.test(message)) return t("Dock settings changed. Review the profile again.");
  if (/capacity/i.test(message)) return t("You can keep up to 8 profiles.");
  if (/invalidDocument/i.test(message)) return t("This launcher profile file is invalid.");
  if (/unavailable/i.test(message)) return t("Profile transfer is unavailable right now.");
  if (/saveFailed/i.test(message)) return t("The imported profile could not be saved.");
  return message;
}

const noticeCopy: Record<string, string> = {
  selectionsRequireRepair: "Files, folders and apps must be selected again on this Mac.",
  widgetsDisabled: "Widgets are imported disabled and without access grants.",
  iconsNotIncluded: "Custom icons are not included.",
};

export function LauncherProfileTransfer({
  profileID,
  t,
  actions,
  onImported,
}: {
  profileID: string;
  t: Translate;
  actions: LauncherProfileTransferActions;
  onImported?: (profileID: string) => void;
}) {
  const [review, setReview] = useState<LauncherProfileImportReview | null>(null);
  const [document, setDocument] = useState("");
  const [pending, setPending] = useState("");
  const [error, setError] = useState("");
  const [exported, setExported] = useState(false);
  const operation = useRef(0);

  useEffect(() => {
    operation.current++;
    setReview(null);
    setDocument("");
    setPending("");
    setError("");
    setExported(false);
    return () => {
      operation.current++;
    };
  }, [profileID]);

  const exportProfile = async () => {
    const owner = ++operation.current;
    setPending("export");
    setError("");
    setExported(false);
    try {
      const result = await actions.exportProfile(profileID);
      if (owner !== operation.current) return;
      setExported(result.status === "saved");
    } catch (cause) {
      if (owner === operation.current) setError(jsonExportError(cause, t));
    } finally {
      if (owner === operation.current) setPending("");
    }
  };

  const choose = async (file?: File) => {
    if (!file) return;
    const owner = ++operation.current;
    setReview(null);
    setDocument("");
    setError("");
    setExported(false);
    if (file.size > MAX_DOCUMENT_BYTES) {
      setError(t("Profile file is larger than 256 KiB."));
      return;
    }
    setPending("review");
    try {
      const exact = await readFileText(file);
      if (owner !== operation.current) return;
      if (new TextEncoder().encode(exact).byteLength > MAX_DOCUMENT_BYTES)
        throw new Error("tooLarge");
      const next = await actions.previewImport(exact);
      if (owner !== operation.current) return;
      setDocument(exact);
      setReview(next);
    } catch (cause) {
      if (owner === operation.current)
        setError(
          cause instanceof Error && cause.message === "tooLarge"
            ? t("Profile file is larger than 256 KiB.")
            : friendlyError(cause, t),
        );
    } finally {
      if (owner === operation.current) setPending("");
    }
  };

  const commit = async () => {
    if (!review || !document) return;
    const owner = ++operation.current;
    const admitted = { document, digest: review.digest, revision: review.revision };
    setPending("import");
    setError("");
    setExported(false);
    try {
      const result = await actions.importProfile(
        admitted.document,
        admitted.digest,
        admitted.revision,
      );
      if (owner !== operation.current) return;
      setReview(null);
      setDocument("");
      onImported?.(result.profileID);
    } catch (cause) {
      if (owner === operation.current) setError(friendlyError(cause, t));
    } finally {
      if (owner === operation.current) setPending("");
    }
  };

  return (
    <section className="space-y-2" aria-label={t("Profile transfer")}>
      <div className={ACTIONS_ROW}>
        <Button disabled={Boolean(pending)} variant="outline" onClick={() => void exportProfile()}>
          {pending === "export" ? t("Exporting…") : t("Export profile")}
        </Button>
        <label>
          <span className="sr-only">{t("Import profile file")}</span>
          <input
            aria-label={t("Import profile file")}
            accept="application/json,.json"
            className="max-w-56 text-sm"
            disabled={Boolean(pending)}
            type="file"
            onChange={(event) => {
              void choose(event.target.files?.[0]);
              event.currentTarget.value = "";
            }}
          />
        </label>
      </div>
      <p className={HINT}>
        {t("Exports omit private access, custom icons and display assignments.")}
      </p>
      {review ? (
        <div
          className="rounded-lg border border-white/12 p-3"
          aria-label={t("Import review")}
          role="region"
        >
          <strong>{review.name}</strong>
          <p>
            {t("{items} items · {widgets} widgets")
              .replace("{items}", String(review.itemCount))
              .replace("{widgets}", String(review.widgetCount))}
          </p>
          <ul className="list-disc pl-5 text-sm">
            {review.notices.map((notice) => (
              <li key={notice}>
                {t(noticeCopy[notice] ?? "The imported profile requires review.")}
              </li>
            ))}
          </ul>
          <div className={ACTIONS_ROW}>
            <Button disabled={Boolean(pending)} onClick={() => void commit()}>
              {pending === "import" ? t("Importing…") : t("Import reviewed profile")}
            </Button>
            <Button
              variant="outline"
              disabled={Boolean(pending)}
              onClick={() => {
                ++operation.current;
                setReview(null);
                setDocument("");
                setError("");
                setExported(false);
              }}
            >
              {t("Cancel import")}
            </Button>
          </div>
        </div>
      ) : null}
      {exported ? <p role="status">{t("Profile exported.")}</p> : null}
      {error ? <p role="alert">{error}</p> : null}
    </section>
  );
}

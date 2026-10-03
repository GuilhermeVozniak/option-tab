import { useEffect, useRef, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import "../settings/editor-settings.css";
import type {
  WidgetCatalogDescriptor,
  WidgetPackageReview,
  WidgetPackageStatus,
} from "../lib/widget-types";
import { capabilityPurpose } from "./WidgetSettings";

export interface WidgetPackageActions {
  review(): Promise<WidgetPackageReview>;
  install(token: string): Promise<WidgetCatalogDescriptor>;
  cancel(token: string): Promise<void>;
  remove(digest: string): Promise<void>;
}

const packageFeedback = {
  invalidFile: "This widget package file is invalid.",
  invalidPackage: "This widget package is invalid.",
  incompatibleVersion: "This widget package requires a newer version of Option Tab.",
  settingsSaveFailed: "Widget settings could not be saved. Try again.",
  busy: "Another package operation is in progress. Try again.",
  unavailable: "Local package management is unavailable.",
  retired: "This package operation is no longer available. Try again.",
  reviewExpired: "This package review expired. Choose the file again.",
  catalogInvalid: "Some installed widget packages could not be loaded.",
  removeFailed: "The widget package could not be removed. Try again.",
  failed: "The widget package operation could not be completed. Try again.",
} as const;
type PackageFeedback = keyof typeof packageFeedback | "";

function packageFeedbackCategory(reason: unknown): PackageFeedback {
  const message = (reason instanceof Error ? reason.message : String(reason))
    .replace(/^Error: /, "")
    .trim();
  if (message === "context canceled") return "";
  const code = message.replace(/^widget package: /, "");
  return Object.hasOwn(packageFeedback, code) ? (code as PackageFeedback) : "failed";
}

export function WidgetPackages({
  catalog,
  status,
  actions,
  onRefresh,
  t = (text) => text,
  language = "en",
}: {
  catalog: WidgetCatalogDescriptor[];
  status: WidgetPackageStatus;
  actions: WidgetPackageActions;
  onRefresh: () => void;
  t?: (text: string) => string;
  language?: string;
}) {
  const [review, setReview] = useState<WidgetPackageReview | null>(null);
  const [pendingReview, setPendingReview] = useState(false);
  const [pendingMutation, setPendingMutation] = useState(false);
  const [error, setError] = useState<PackageFeedback>("");
  const owner = useRef(0);
  useEffect(() => {
    if (status.available) return;
    owner.current++;
    setReview(null);
    setPendingReview(false);
    setPendingMutation(false);
  }, [status.available]);
  useEffect(() => () => void owner.current++, []);
  const fail = (reason: unknown) => {
    setError(packageFeedbackCategory(reason));
  };
  const statusFeedback = status.reason ? packageFeedbackCategory(status.reason) : "";
  const isAction = (capability: string) =>
    capability.endsWith(".control") || capability.endsWith(".select");
  return (
    <section className="ot-widget-packages ot-settings-editor" aria-label={t("Widget packages")}>
      <header>
        <div className="ot-editor-heading">
          <h3>{t("Widget packages")}</h3>
          <p>{t("Review a local widget package before installing it.")}</p>
        </div>
        <Button
          type="button"
          disabled={
            !status.available || status.busy || pendingReview || pendingMutation || !!review
          }
          onClick={() => {
            const operation = ++owner.current;
            setPendingReview(true);
            setError("");
            void actions
              .review()
              .then((value) => {
                if (owner.current === operation) setReview(value);
              })
              .catch((reason) => {
                const message = reason instanceof Error ? reason.message : String(reason);
                if (owner.current === operation && message !== "context canceled") fail(reason);
              })
              .finally(() => owner.current === operation && setPendingReview(false));
          }}
        >
          {pendingReview ? t("Waiting for file…") : t("Review local package…")}
        </Button>
      </header>
      {pendingReview ? (
        <Button
          type="button"
          onClick={() => {
            const operation = ++owner.current;
            setPendingReview(false);
            void actions.cancel("").catch((reason) => owner.current === operation && fail(reason));
          }}
        >
          {t("Cancel review")}
        </Button>
      ) : null}
      {review ? (
        <article className="ot-widget-package-review">
          <h4>{review.package.name[language] || review.package.name.en}</h4>
          <p>{review.package.description[language] || review.package.description.en}</p>
          <dl>
            <div>
              <dt>{t("File")}</dt>
              <dd>{review.sourceName}</dd>
            </div>
            <div>
              <dt>{t("Version")}</dt>
              <dd>{review.package.version}</dd>
            </div>
            <div>
              <dt>{t("Publisher")}</dt>
              <dd>{t("Not independently verified")}</dd>
            </div>
            <div>
              <dt>{t("Data access")}</dt>
              <dd>
                {[...review.package.requiredCapabilities, ...review.package.optionalCapabilities]
                  .filter((capability) => !isAction(capability))
                  .map((capability) => capabilityPurpose(capability, language))
                  .join(", ") || t("No data access")}
              </dd>
            </div>
            <div>
              <dt>{t("Actions")}</dt>
              <dd>
                {[...review.package.requiredCapabilities, ...review.package.optionalCapabilities]
                  .filter(isAction)
                  .map((capability) => capabilityPurpose(capability, language))
                  .join(", ") || t("No actions")}
              </dd>
            </div>
          </dl>
          <p>{t("Installing does not grant data access or actions.")}</p>
          {review.alreadyInstalled ? <p>{t("This exact package is already installed.")}</p> : null}
          <div className="ot-widget-settings-buttons">
            <Button
              type="button"
              variant="default"
              disabled={status.busy || pendingMutation}
              onClick={() => {
                const operation = ++owner.current;
                const token = review.token;
                setError("");
                setPendingMutation(true);
                void actions
                  .install(token)
                  .then(() => {
                    if (owner.current !== operation) return;
                    setReview(null);
                    onRefresh();
                  })
                  .catch((reason) => owner.current === operation && fail(reason))
                  .finally(() => owner.current === operation && setPendingMutation(false));
              }}
            >
              {t("Install reviewed package")}
            </Button>
            <Button
              type="button"
              disabled={status.busy || pendingMutation}
              onClick={() => {
                const token = review.token;
                const operation = ++owner.current;
                setReview(null);
                void actions
                  .cancel(token)
                  .catch((reason) => owner.current === operation && fail(reason));
              }}
            >
              {t("Close review")}
            </Button>
          </div>
        </article>
      ) : null}
      {catalog.some((item) => !item.builtin) ? (
        <ul className="ot-widget-package-list">
          {catalog
            .filter((item) => !item.builtin)
            .map((item) => (
              <li key={item.digest}>
                <span>
                  {item.name[language] || item.name.en} · {item.version}
                </span>
                <Button
                  type="button"
                  disabled={!status.available || status.busy || pendingReview || pendingMutation}
                  onClick={() => {
                    const operation = ++owner.current;
                    setError("");
                    setPendingMutation(true);
                    void actions
                      .remove(item.digest)
                      .then(() => owner.current === operation && onRefresh())
                      .catch((reason) => owner.current === operation && fail(reason))
                      .finally(() => owner.current === operation && setPendingMutation(false));
                  }}
                >
                  {t("Remove package")}
                </Button>
              </li>
            ))}
        </ul>
      ) : null}
      {!status.available ? <p>{t("Local package management is unavailable.")}</p> : null}
      {statusFeedback && (status.available || statusFeedback !== "unavailable") ? (
        <p role="status">{t(packageFeedback[statusFeedback])}</p>
      ) : null}
      {error ? (
        <Alert appearance="unstyled" asChild>
          <p role="alert">{t(packageFeedback[error])}</p>
        </Alert>
      ) : null}
    </section>
  );
}

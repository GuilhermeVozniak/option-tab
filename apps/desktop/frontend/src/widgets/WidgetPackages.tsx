import { useEffect, useRef, useState } from "react";
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
  const [error, setError] = useState("");
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
    const message = String(reason);
    setError(
      message.includes("reviewExpired")
        ? t("This package review expired. Choose the file again.")
        : message,
    );
  };
  const isAction = (capability: string) =>
    capability.endsWith(".control") || capability.endsWith(".select");
  return (
    <section className="ot-widget-packages" aria-label={t("Widget packages")}>
      <header>
        <div>
          <strong>{t("Widget packages")}</strong>
          <p>{t("Review a local widget package before installing it.")}</p>
        </div>
        <button
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
        </button>
      </header>
      {pendingReview ? (
        <button
          type="button"
          onClick={() => {
            owner.current++;
            setPendingReview(false);
            void actions.cancel("").catch(fail);
          }}
        >
          {t("Cancel review")}
        </button>
      ) : null}
      {review ? (
        <article className="ot-widget-package-review">
          <h5>{review.package.name[language] || review.package.name.en}</h5>
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
            <button
              type="button"
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
            </button>
            <button
              type="button"
              disabled={status.busy || pendingMutation}
              onClick={() => {
                const token = review.token;
                owner.current++;
                setReview(null);
                void actions.cancel(token).catch(fail);
              }}
            >
              {t("Close review")}
            </button>
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
                <button
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
                </button>
              </li>
            ))}
        </ul>
      ) : null}
      {!status.available ? <p>{t("Local package management is unavailable.")}</p> : null}
      {status.reason ? <p role="status">{status.reason}</p> : null}
      {error ? <p role="alert">{error}</p> : null}
    </section>
  );
}

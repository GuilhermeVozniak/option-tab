import type { WindowActionResult } from "./bridge";

export type ActionErrorReason = { key: string; count?: number };
export interface ActionFeedback {
  accepted: number;
  failures: Array<{ windowId: number; reason: ActionErrorReason }>;
}

const incomplete = "Some windows could not be checked. Results may be incomplete.";

// Keep only bounded reason keys and safe numeric context. Native error text can
// contain private details and is never used as user-facing copy.
export function normalizeActionError(error: unknown, kind = ""): ActionErrorReason {
  const message = (error instanceof Error ? error.message : String(error)).slice(0, 2048);
  let key = "The window action could not be completed. Try again.";
  if (/window enumeration incomplete/i.test(message)) {
    const count = Number(message.match(/incomplete:\s*(\d+)\s*candidate/i)?.[1]);
    if (Number.isSafeInteger(count) && count > 0)
      return {
        key:
          count === 1
            ? "1 possible window could not be checked. Results may be incomplete."
            : "{count} possible windows could not be checked. Results may be incomplete.",
        count,
      };
    key = incomplete;
  } else if (/window enumeration is unavailable or refused/i.test(message)) {
    key = "The app's windows could not be checked.";
  } else if (/folderOpenRefused/i.test(message)) {
    key = "The item could not be opened.";
  } else if (
    /previewUnavailable|window preview unavailable|launcher:.*(?:retired|stale)|(?:dock:? )?preview session is no longer active/i.test(
      message,
    )
  ) {
    key = "This preview is no longer available.";
  } else if (/launcher:.*busy/i.test(message)) {
    key = "The launcher is busy. Try again.";
  } else if (/accessibility.*(?:permission|required|denied)/i.test(message)) {
    key = "Accessibility permission is required for this action.";
  } else if (/new window.*(?:command|unsupported)|supported.*new window/i.test(message)) {
    key = "The app does not expose an available New Window command.";
  } else if (/requested AX action is unsupported/i.test(message)) {
    key = "This action is not supported for this window.";
  } else if (/unsupported|not supported/i.test(message)) {
    key = "This action is not supported by the application.";
  } else if (
    /switcher.*(?:no longer current|retired)|not visible in this switcher session/i.test(message)
  ) {
    key = "The switcher changed. Try again.";
  } else if (
    /no longer (?:exists|available|running)|identity (?:mismatch|changed)|invalid or self.*identity/i.test(
      message,
    )
  ) {
    key = "This window or application is no longer available.";
  } else if (/refused|could not be (?:acknowledged|completed|confirmed)/i.test(message)) {
    key =
      kind === "close" || kind === "closeAll" || /\bclose\b/i.test(message)
        ? "The close request was refused or could not be confirmed. Check the app for a save dialog."
        : "The action was refused or could not be confirmed.";
  }
  return { key };
}

export function formatActionError(reason: ActionErrorReason, t: (key: string) => string): string {
  return t(reason.key).replace("{count}", () => String(reason.count ?? 0));
}

export function actionFailureFeedback(error: unknown, kind = "", windowId = 0): ActionFeedback {
  const target = kind === "closeAll" || kind === "minimizeAll" ? 0 : windowId;
  return {
    accepted: 0,
    failures: [{ windowId: target, reason: normalizeActionError(error, kind) }],
  };
}

export function actionResultFeedback(kind: string, result: WindowActionResult): ActionFeedback {
  const bulk = kind === "closeAll" || kind === "minimizeAll";
  return {
    accepted: result.succeeded,
    failures: result.failures.map(({ windowId, error }) => {
      let reason = normalizeActionError(error, kind);
      // The action service reserves ID zero for incomplete bulk enumeration.
      // Even an unfamiliar lookup error must not erase that warning.
      if (bulk && windowId === 0 && reason.count === undefined) reason = { key: incomplete };
      return { windowId, reason };
    }),
  };
}

export function formatActionFeedback(feedback: ActionFeedback, t: (key: string) => string): string {
  const parts: string[] = [];
  if (feedback.accepted > 0)
    parts.push(
      t(
        feedback.accepted === 1
          ? "1 action request accepted."
          : "{count} action requests accepted.",
      ).replace("{count}", () => String(feedback.accepted)),
    );
  for (const failure of feedback.failures) {
    const reason = formatActionError(failure.reason, t);
    parts.push(
      failure.windowId > 0
        ? t("Window {id}: {reason}")
            .replace("{id}", () => String(failure.windowId))
            .replace("{reason}", () => reason)
        : reason,
    );
  }
  return parts.join(" ");
}

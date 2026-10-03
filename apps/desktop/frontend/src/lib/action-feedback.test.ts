import { describe, expect, it } from "vitest";
import {
  actionFailureFeedback,
  actionResultFeedback,
  formatActionError,
  formatActionFeedback,
  normalizeActionError,
} from "./action-feedback";
import { makeT } from "./i18n";

describe("action feedback", () => {
  it("keeps accepted requests, enumeration uncertainty and every target failure separate", () => {
    const feedback = actionResultFeedback("closeAll", {
      succeeded: 3,
      failures: [
        {
          windowId: 0,
          error:
            "window enumeration incomplete: 9 candidate(s) could not be classified because AX lookup was unavailable, refused, or timed out",
        },
        { windowId: 42, error: "close was refused or could not be completed by the application" },
        { windowId: 45, error: "window no longer exists or application identity mismatches" },
      ],
    });
    const text = formatActionFeedback(feedback, makeT("en"));
    expect(text).toContain("3 action requests accepted.");
    expect(text).toContain("9 possible windows could not be checked. Results may be incomplete.");
    expect(text).toContain("Window 42: The close request was refused or could not be confirmed.");
    expect(text).toContain("Window 45: This window or application is no longer available.");
    expect(text).not.toMatch(/windows closed|AX lookup|identity mismatches/);
    expect(formatActionFeedback(feedback, makeT("es"))).toContain("Ventana 42:");
  });

  it("keeps an unrecognized snapshot-wide bulk error as an incomplete-results warning", () => {
    const text = formatActionFeedback(
      actionResultFeedback("minimizeAll", {
        succeeded: 1,
        failures: [{ windowId: 0, error: "private lookup details" }],
      }),
      makeT("en"),
    );
    expect(text).toBe(
      "1 action request accepted. Some windows could not be checked. Results may be incomplete.",
    );
    expect(
      formatActionFeedback(
        actionFailureFeedback("application window enumeration is unavailable or refused"),
        makeT("en"),
      ),
    ).toBe("The app's windows could not be checked.");
  });

  it.each([
    [
      "accessibility permission is required",
      "Accessibility permission is required for this action.",
    ],
    ["requested AX action is unsupported", "This action is not supported for this window."],
    [
      "force quit is unsupported by this platform",
      "This action is not supported by the application.",
    ],
    [
      "application does not expose a supported, enabled New Window menu command",
      "The app does not expose an available New Window command.",
    ],
    ["switcher session is no longer current", "The switcher changed. Try again."],
    ["application is no longer running", "This window or application is no longer available."],
    [
      "minimize was refused or could not be acknowledged by the target window",
      "The action was refused or could not be confirmed.",
    ],
    ["launcher: retired scope", "This preview is no longer available."],
    ["launcher: action busy", "The launcher is busy. Try again."],
    ["previewUnavailable", "This preview is no longer available."],
    ["Window preview unavailable", "This preview is no longer available."],
    ["dock: preview session is no longer active", "This preview is no longer available."],
    ["folderOpenRefused", "The item could not be opened."],
  ])("normalizes %s without exposing native details", (error, expected) => {
    expect(formatActionError(normalizeActionError(new Error(error)), makeT("en"))).toBe(expected);
  });

  it("uses a bounded generic message for unknown errors and malformed counts", () => {
    expect(
      formatActionError(
        normalizeActionError("/Users/private/file " + "x".repeat(10000)),
        makeT("pt-BR"),
      ),
    ).toBe("Não foi possível concluir a ação na janela. Tente novamente.");
    expect(
      formatActionError(
        normalizeActionError("window enumeration incomplete: 999999999999999999999 candidate(s)"),
        makeT("en"),
      ),
    ).toBe("Some windows could not be checked. Results may be incomplete.");
  });

  it("does not label an app-wide bulk lookup failure with the selected window's ID", () => {
    expect(
      formatActionFeedback(
        actionFailureFeedback(
          "application window enumeration is unavailable or refused",
          "closeAll",
          42,
        ),
        makeT("en"),
      ),
    ).toBe("The app's windows could not be checked.");
  });
});

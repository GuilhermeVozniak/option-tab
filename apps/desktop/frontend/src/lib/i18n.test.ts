import { describe, expect, it } from "vitest";
import { LANGUAGES, makeT, resolveLang, TRANSLATIONS } from "./i18n";

describe("i18n", () => {
  it("resolves explicit languages and falls back to English", () => {
    expect(resolveLang("en")).toBe("en");
    expect(resolveLang("pt-BR")).toBe("pt-BR");
    expect(resolveLang("es")).toBe("es");
    expect(resolveLang("fr")).toBe("en"); // unsupported explicit value
  });

  it("translates known strings and falls back for unknown ones", () => {
    const t = makeT("pt-BR");
    expect(t("Start at login")).toBe("Iniciar no login");
    expect(t("totally unknown string")).toBe("totally unknown string");
    const es = makeT("es");
    expect(es("Start at login")).toBe("Iniciar al iniciar sesión");
  });

  it("English is identity", () => {
    const t = makeT("en");
    expect(t("Start at login")).toBe("Start at login");
  });

  it("sniffs navigator.language for the system default", () => {
    const original = Object.getOwnPropertyDescriptor(Navigator.prototype, "language");
    const sniff = (navLang: string) => {
      Object.defineProperty(navigator, "language", { value: navLang, configurable: true });
      return resolveLang("");
    };
    try {
      expect(sniff("pt-PT")).toBe("pt-BR");
      expect(sniff("es-MX")).toBe("es");
      expect(sniff("de-DE")).toBe("en");
    } finally {
      delete (navigator as { language?: string }).language;
      if (original && !("language" in Navigator.prototype)) {
        Object.defineProperty(Navigator.prototype, "language", original);
      }
    }
  });

  it("offers system default plus the supported languages", () => {
    expect(LANGUAGES.map((l) => l.value)).toEqual(["", "en", "pt-BR", "es"]);
  });

  it("defines every new icon-only and platform control in both dictionaries", () => {
    const keys = [
      "Previous",
      "Capture windows in the background",
      "Keeps thumbnails fresh so the switcher opens with previews instantly. While enabled, macOS shows the screen-recording indicator.",
      "Play",
      "Pause",
      "Next",
      "Playback position",
      "Pin media panel",
      "Close media panel",
      "Option",
      "Control",
      "Command",
      "Shift",
      "Close preview",
      "Diagnostics",
      "Review diagnostics",
      "Refresh preview",
      "Start recording",
      "Stop recording",
      "Clear diagnostics",
      "Save report…",
      "Diagnostics report preview",
      "Report saved",
      "Focus rules",
      "Add focus rule",
      "Running app",
      "Exact bundle identifier",
      "Destination profile",
      "Display scope",
      "Every assigned display",
      "Enter an exact bundle identifier using letters, numbers, dots, hyphens, or underscores.",
      "Removing a display assignment also removes focus rules scoped only to that display.",
    ];
    for (const locale of ["pt-BR", "es"] as const) {
      for (const key of keys) {
        expect(TRANSLATIONS[locale], `${locale} is missing ${key}`).toHaveProperty(key);
        expect(TRANSLATIONS[locale][key]).not.toBe("");
      }
    }
    expect(TRANSLATIONS["pt-BR"].Shift).toBe("Shift");
    expect(TRANSLATIONS.es.Shift).toBe("Mayúsculas");
  });
});

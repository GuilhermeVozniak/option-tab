import { readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";
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

  it("defines the literal translation keys used by production source in both dictionaries", () => {
    const keys = new Map<string, string>();
    const literals = (node: ts.Node): string[] => {
      if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return [node.text];
      if (ts.isConditionalExpression(node))
        return [...literals(node.whenTrue), ...literals(node.whenFalse)];
      return [];
    };
    const inspectDirectory = (directory: string) => {
      for (const entry of readdirSync(directory, { withFileTypes: true })) {
        const path = join(directory, entry.name);
        if (entry.isDirectory()) {
          if (entry.name !== "test") inspectDirectory(path);
          continue;
        }
        if (!/\.tsx?$/.test(entry.name) || /\.test\.tsx?$/.test(entry.name)) continue;
        const source = ts.createSourceFile(
          path,
          readFileSync(path, "utf8"),
          ts.ScriptTarget.Latest,
          true,
        );
        const visit = (node: ts.Node) => {
          if (
            ts.isCallExpression(node) &&
            ((ts.isIdentifier(node.expression) && node.expression.text === "t") ||
              (ts.isPropertyAccessExpression(node.expression) &&
                node.expression.name.text === "t")) &&
            node.arguments[0]
          ) {
            const line = source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1;
            for (const key of literals(node.arguments[0])) keys.set(key, `${path}:${line}`);
          }
          ts.forEachChild(node, visit);
        };
        visit(source);
      }
    };
    inspectDirectory(join(dirname(fileURLToPath(import.meta.url)), ".."));
    expect(keys.size, "No production translation calls were inspected").toBeGreaterThan(0);
    for (const locale of ["pt-BR", "es"] as const) {
      const missing = [...keys]
        .filter(
          ([key]) => !Object.hasOwn(TRANSLATIONS[locale], key) || !TRANSLATIONS[locale][key].trim(),
        )
        .map(([key, path]) => ({ key, path }));
      expect(missing, `${locale} leaves production controls untranslated`).toEqual([]);
    }
  });

  it("retains coverage for selected controls whose keys may be chosen dynamically", () => {
    const keys = [
      "This widget package file is invalid.",
      "This widget package is invalid.",
      "This widget package requires a newer version of Option Tab.",
      "Widget settings could not be saved. Try again.",
      "Another package operation is in progress. Try again.",
      "This package operation is no longer available. Try again.",
      "Some installed widget packages could not be loaded.",
      "The widget package could not be removed. Try again.",
      "The widget package operation could not be completed. Try again.",
      "Local package management is unavailable.",
      "This package review expired. Choose the file again.",
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

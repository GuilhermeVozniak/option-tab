import { describe, expect, it } from "vitest";
import { LANGUAGES, makeT, resolveLang } from "./i18n";

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

  it("offers system default plus the supported languages", () => {
    expect(LANGUAGES.map((l) => l.value)).toEqual(["", "en", "pt-BR", "es"]);
  });
});

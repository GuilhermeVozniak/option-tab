import { useMemo, useState } from "react";
import type {
  WidgetCatalogDescriptor,
  WidgetInstanceConfig,
  WidgetLocalized,
  WidgetSetting,
  WidgetStackConfig,
  WidgetValue,
} from "../lib/widget-types";

export interface WidgetSettingsProps {
  instances: WidgetInstanceConfig[];
  stacks: WidgetStackConfig[];
  catalog: WidgetCatalogDescriptor[];
  onChange: (value: { widgets: WidgetInstanceConfig[]; stacks: WidgetStackConfig[] }) => void;
  language?: "en" | "pt-BR" | "es";
  t?: (text: string) => string;
}

const legacyClockDigest = "sha256:09bcb4221f7ace94e21898b4e583f9db72be1be5f6524fbe9972af70726c9dc3";

const copy = {
  en: {
    package: "Widget package",
    add: "Add widget",
    widgets: "Widgets",
    enabled: "Enable",
    remove: "Remove",
    up: "Move up",
    down: "Move down",
    required: "Required",
    optional: "Optional",
    unavailable: "Unavailable until required access is granted.",
    stacks: "Stacks",
    stackName: "Stack name",
    createStack: "Create stack",
    dissolve: "Dissolve",
    active: "Shown widget",
    stack: "Stack",
    invalid: "Enter a valid value.",
  },
  "pt-BR": {
    package: "Pacote do widget",
    add: "Adicionar widget",
    widgets: "Widgets",
    enabled: "Ativar",
    remove: "Remover",
    up: "Mover para cima",
    down: "Mover para baixo",
    required: "Obrigatório",
    optional: "Opcional",
    unavailable: "Indisponível até que o acesso obrigatório seja concedido.",
    stacks: "Pilhas",
    stackName: "Nome da pilha",
    createStack: "Criar pilha",
    dissolve: "Desfazer pilha",
    active: "Widget exibido",
    stack: "Pilha",
    invalid: "Insira um valor válido.",
  },
  es: {
    package: "Paquete del widget",
    add: "Añadir widget",
    widgets: "Widgets",
    enabled: "Activar",
    remove: "Eliminar",
    up: "Mover arriba",
    down: "Mover abajo",
    required: "Obligatorio",
    optional: "Opcional",
    unavailable: "No disponible hasta conceder el acceso obligatorio.",
    stacks: "Pilas",
    stackName: "Nombre de la pila",
    createStack: "Crear pila",
    dissolve: "Disolver pila",
    active: "Widget mostrado",
    stack: "Pila",
    invalid: "Introduce un valor válido.",
  },
} as const;

const capabilityCopy: Record<string, Record<string, string>> = {
  "clock.read": { en: "Read local time", "pt-BR": "Ler a hora local", es: "Leer la hora local" },
  "battery.read": {
    en: "Read battery status",
    "pt-BR": "Ler o estado da bateria",
    es: "Leer el estado de la batería",
  },
  "network.status.read": {
    en: "Network status",
    "pt-BR": "Estado da rede",
    es: "Estado de la red",
  },
  "network.usage.read": { en: "Network usage", "pt-BR": "Uso da rede", es: "Uso de la red" },
  "audio.status.read": { en: "Audio status", "pt-BR": "Estado do áudio", es: "Estado del audio" },
  "audio.output.select": {
    en: "Choose audio output",
    "pt-BR": "Escolher saída de áudio",
    es: "Elegir salida de audio",
  },
  "media.music.read": {
    en: "Read Music playback",
    "pt-BR": "Ler reprodução do Música",
    es: "Leer reproducción de Música",
  },
  "media.spotify.read": {
    en: "Read Spotify playback",
    "pt-BR": "Ler reprodução do Spotify",
    es: "Leer reproducción de Spotify",
  },
  "media.music.control": {
    en: "Control Music",
    "pt-BR": "Controlar Música",
    es: "Controlar Música",
  },
  "media.spotify.control": {
    en: "Control Spotify",
    "pt-BR": "Controlar Spotify",
    es: "Controlar Spotify",
  },
};
export function capabilityPurpose(capability: string, language: string): string {
  return capabilityCopy[capability]?.[language] ?? "Local app data";
}

function localized(value: WidgetLocalized, language: string): string {
  return value[language] || value.en || Object.values(value)[0] || "Widget";
}
const choiceLabels: Record<string, Record<string, string>> = {
  shortTime: { en: "Short time", "pt-BR": "Hora curta", es: "Hora corta" },
  longTime: { en: "Long time", "pt-BR": "Hora completa", es: "Hora completa" },
  date: { en: "Date", "pt-BR": "Data", es: "Fecha" },
};
function choiceLabel(value: string, language: string): string {
  return choiceLabels[value]?.[language] ?? value;
}

function cloneWidgets(values: WidgetInstanceConfig[]): WidgetInstanceConfig[] {
  return values.map((value) => ({
    ...value,
    grants: [...value.grants],
    settings: value.settings ? structuredClone(value.settings) : undefined,
  }));
}
function cloneStacks(values: WidgetStackConfig[]): WidgetStackConfig[] {
  return values.map((value) => ({ ...value, members: [...value.members] }));
}
function defaultSettings(settings: WidgetSetting[]): Record<string, WidgetValue> | undefined {
  const result: Record<string, WidgetValue> = {};
  for (const setting of settings) {
    if (setting.defaultText !== undefined) result[setting.id] = { text: setting.defaultText };
    else if (setting.defaultNumber !== undefined)
      result[setting.id] = { number: setting.defaultNumber };
    else if (setting.defaultBool !== undefined)
      result[setting.id] = { boolean: setting.defaultBool };
  }
  return Object.keys(result).length ? result : undefined;
}
function makeID(base: string, used: Set<string>): string {
  const stem =
    base
      .split(".")
      .at(-1)
      ?.replace(/[^a-z0-9._-]/g, "-") || "widget";
  if (!used.has(stem)) return stem;
  for (let number = 2; number <= 99; number++)
    if (!used.has(`${stem}-${number}`)) return `${stem}-${number}`;
  return `widget-${Date.now()}`;
}

export function WidgetSettings({
  instances,
  stacks,
  catalog,
  onChange,
  language = "en",
  t,
}: WidgetSettingsProps) {
  const c = copy[language];
  const tr = (text: string) => (t ? t(text) : text);
  const [packageDigest, setPackageDigest] = useState(catalog[0]?.digest ?? "");
  const [stackMembers, setStackMembers] = useState<string[]>([]);
  const [stackName, setStackName] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const byDigest = useMemo(() => new Map(catalog.map((item) => [item.digest, item])), [catalog]);
  const selectedDigest = byDigest.has(packageDigest) ? packageDigest : (catalog[0]?.digest ?? "");
  const builtinClock = catalog.find(
    (item) => item.builtin && item.packageID === "org.optiontab.clock",
  );
  const emit = (widgets: WidgetInstanceConfig[], nextStacks = stacks) =>
    onChange({ widgets: cloneWidgets(widgets), stacks: cloneStacks(nextStacks) });
  const descriptorFor = (instance: WidgetInstanceConfig) => {
    if (instance.packageID === "org.optiontab.clock" && instance.digest === legacyClockDigest)
      return builtinClock;
    const item = byDigest.get(instance.digest);
    return item?.packageID === instance.packageID ? item : undefined;
  };
  const editable = (instance: WidgetInstanceConfig): WidgetInstanceConfig => {
    if (instance.packageID !== "org.optiontab.clock" || instance.digest !== legacyClockDigest)
      return {
        ...instance,
        grants: [...instance.grants],
        settings: structuredClone(instance.settings),
      };
    const current = builtinClock;
    if (!current) return { ...instance, grants: [...instance.grants] };
    return {
      ...instance,
      digest: current.digest,
      grants: instance.grants.filter((grant) => grant === "clock.read"),
      settings: undefined,
    };
  };
  const slots =
    instances.length - new Set(stacks.flatMap((stack) => stack.members)).size + stacks.length;
  const selectedMembers = stackMembers.filter(
    (id) =>
      instances.some((instance) => instance.id === id) &&
      !stacks.some((stack) => stack.members.includes(id)),
  );
  const projectedSlots = slots - selectedMembers.length + (selectedMembers.length >= 2 ? 1 : 0);

  const updateSetting = (instance: WidgetInstanceConfig, setting: WidgetSetting, raw: unknown) => {
    let value: WidgetValue | null = null;
    if (setting.type === "boolean") value = { boolean: Boolean(raw) };
    if (setting.type === "number") {
      const number = Number(raw);
      if (
        Number.isFinite(number) &&
        number >= (setting.min ?? -1e9) &&
        number <= (setting.max ?? 1e9)
      )
        value = { number };
    }
    if (setting.type === "choice" && setting.options?.includes(String(raw)))
      value = { text: String(raw) };
    if (setting.type === "timezone" && String(raw).trim() && String(raw).length <= 80)
      value = { text: String(raw) };
    const key = `${instance.id}:${setting.id}`;
    if (!value) {
      setErrors((current) => ({ ...current, [key]: c.invalid }));
      return;
    }
    setErrors((current) => ({ ...current, [key]: "" }));
    const next = cloneWidgets(instances);
    const target = editable(next.find((item) => item.id === instance.id) ?? instance);
    target.settings = { ...(target.settings ?? {}), [setting.id]: value };
    emit(next.map((item) => (item.id === instance.id ? target : item)));
  };

  return (
    <section className="ot-widget-settings" aria-label={tr(c.widgets)}>
      <div className="ot-widget-settings-add">
        <label>
          {tr(c.package)}
          <select
            aria-label={tr(c.package)}
            value={selectedDigest}
            onChange={(event) => setPackageDigest(event.target.value)}
          >
            {catalog.map((item) => (
              <option key={`${item.packageID}:${item.digest}`} value={item.digest}>
                {localized(item.name, language)} · {item.version}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          disabled={!byDigest.has(selectedDigest) || slots >= 4 || instances.length >= 16}
          onClick={() => {
            const item = byDigest.get(selectedDigest);
            if (!item) return;
            emit([
              ...instances,
              {
                id: makeID(item.packageID, new Set(instances.map((instance) => instance.id))),
                packageID: item.packageID,
                digest: item.digest,
                enabled: false,
                grants: [],
                settings: defaultSettings(item.settings),
              },
            ]);
          }}
        >
          {tr(c.add)}
        </button>
      </div>

      <div className="ot-widget-settings-list">
        {instances.map((instance, index) => {
          const item = descriptorFor(instance);
          const name = item ? localized(item.name, language) : instance.packageID;
          const requiredMissing = item?.requiredCapabilities.some(
            (capability) => !instance.grants.includes(capability),
          );
          return (
            <article className="ot-widget-settings-card" key={instance.id}>
              <header>
                <strong>{name}</strong>
                <span>{item?.version}</span>
              </header>
              <p>{item ? localized(item.description, language) : instance.packageID}</p>
              <label>
                <input
                  aria-label={`${tr(c.enabled)} ${name}`}
                  type="checkbox"
                  checked={instance.enabled}
                  onChange={(event) => {
                    const next = cloneWidgets(instances);
                    next[index] = { ...editable(next[index]), enabled: event.target.checked };
                    emit(next);
                  }}
                />
                {tr(c.enabled)}
              </label>
              {item
                ? [...item.requiredCapabilities, ...item.optionalCapabilities].map((capability) => {
                    const required = item.requiredCapabilities.includes(capability);
                    const purpose = capabilityPurpose(capability, language);
                    return (
                      <label className="ot-widget-capability" key={capability}>
                        <input
                          type="checkbox"
                          checked={instance.grants.includes(capability)}
                          onChange={(event) => {
                            const next = cloneWidgets(instances);
                            const target = editable(next[index]);
                            target.grants = event.target.checked
                              ? [...target.grants, capability]
                              : target.grants.filter((grant) => grant !== capability);
                            next[index] = target;
                            emit(next);
                          }}
                        />
                        {tr(required ? c.required : c.optional)} · {purpose}
                      </label>
                    );
                  })
                : null}
              {requiredMissing ? (
                <p className="ot-widget-settings-note">{tr(c.unavailable)}</p>
              ) : null}
              {item?.settings.map((setting) => {
                const key = `${instance.id}:${setting.id}`;
                const configured = instance.settings?.[setting.id];
                const label = localized(setting.name, language);
                const text =
                  configured && "text" in configured
                    ? configured.text
                    : (setting.defaultText ?? "");
                const number =
                  configured && "number" in configured
                    ? configured.number
                    : (setting.defaultNumber ?? 0);
                const checked =
                  configured && "boolean" in configured
                    ? configured.boolean
                    : (setting.defaultBool ?? false);
                return (
                  <label className="ot-widget-setting" key={setting.id}>
                    {label}
                    {setting.type === "choice" ? (
                      <select
                        aria-label={label}
                        value={text}
                        onChange={(event) => updateSetting(instance, setting, event.target.value)}
                      >
                        {setting.options?.map((option) => (
                          <option key={option} value={option}>
                            {choiceLabel(option, language)}
                          </option>
                        ))}
                      </select>
                    ) : setting.type === "boolean" ? (
                      <input
                        aria-label={label}
                        type="checkbox"
                        checked={checked}
                        onChange={(event) => updateSetting(instance, setting, event.target.checked)}
                      />
                    ) : (
                      <input
                        aria-label={label}
                        type={setting.type === "number" ? "number" : "text"}
                        value={setting.type === "number" ? number : text}
                        min={setting.min}
                        max={setting.max}
                        onChange={(event) => updateSetting(instance, setting, event.target.value)}
                      />
                    )}
                    {errors[key] ? <span role="alert">{tr(errors[key])}</span> : null}
                  </label>
                );
              })}
              <div className="ot-widget-settings-buttons">
                <button
                  type="button"
                  aria-label={`${tr(c.up)} ${name}`}
                  disabled={index === 0}
                  onClick={() => {
                    const next = cloneWidgets(instances);
                    [next[index - 1], next[index]] = [next[index], next[index - 1]];
                    emit(next);
                  }}
                >
                  ↑
                </button>
                <button
                  type="button"
                  aria-label={`${tr(c.down)} ${name}`}
                  disabled={index === instances.length - 1}
                  onClick={() => {
                    const next = cloneWidgets(instances);
                    [next[index], next[index + 1]] = [next[index + 1], next[index]];
                    emit(next);
                  }}
                >
                  ↓
                </button>
                <button
                  type="button"
                  aria-label={`${tr(c.remove)} ${name}`}
                  onClick={() => {
                    const widgets = instances.filter((candidate) => candidate.id !== instance.id);
                    const nextStacks = stacks.flatMap((stack) => {
                      const members = stack.members.filter((id) => id !== instance.id);
                      return members.length < 2
                        ? []
                        : [
                            {
                              ...stack,
                              members,
                              activeID: members.includes(stack.activeID)
                                ? stack.activeID
                                : members[0],
                            },
                          ];
                    });
                    emit(widgets, nextStacks);
                  }}
                >
                  {tr(c.remove)}
                </button>
              </div>
              {!stacks.some((stack) => stack.members.includes(instance.id)) ? (
                <label>
                  <input
                    aria-label={`${tr(c.stack)} ${instance.id}`}
                    type="checkbox"
                    checked={selectedMembers.includes(instance.id)}
                    onChange={(event) =>
                      setStackMembers(
                        event.target.checked
                          ? [...selectedMembers, instance.id].slice(0, 4)
                          : selectedMembers.filter((id) => id !== instance.id),
                      )
                    }
                  />
                  {tr(c.stack)}
                </label>
              ) : null}
            </article>
          );
        })}
      </div>

      <section className="ot-widget-stacks">
        <h4>{tr(c.stacks)}</h4>
        <label>
          {tr(c.stackName)}
          <input
            aria-label={tr(c.stackName)}
            maxLength={80}
            value={stackName}
            onChange={(event) => setStackName(event.target.value)}
          />
        </label>
        <button
          type="button"
          disabled={selectedMembers.length < 2 || stacks.length >= 4 || projectedSlots > 4}
          onClick={() => {
            const id = makeID("stack", new Set(stacks.map((stack) => stack.id)));
            emit(instances, [
              ...stacks,
              {
                id,
                name: stackName.trim() || `${tr(c.stack)} ${stacks.length + 1}`,
                members: [...selectedMembers],
                activeID: selectedMembers[0],
              },
            ]);
            setStackMembers([]);
            setStackName("");
          }}
        >
          {tr(c.createStack)}
        </button>
        {stacks.map((stack, index) => (
          <article className="ot-widget-stack" key={stack.id}>
            <input
              aria-label={`${tr(c.stackName)} ${index + 1}`}
              maxLength={80}
              defaultValue={stack.name}
              onBlur={(event) => {
                const name = event.target.value.trim();
                if (!name || name === stack.name) return;
                emit(
                  instances,
                  stacks.map((item) => (item.id === stack.id ? { ...item, name } : item)),
                );
              }}
            />
            <label>
              {tr(c.active)}
              <select
                value={stack.activeID}
                onChange={(event) =>
                  emit(
                    instances,
                    stacks.map((item) =>
                      item.id === stack.id ? { ...item, activeID: event.target.value } : item,
                    ),
                  )
                }
              >
                {stack.members.map((id) => (
                  <option key={id}>{id}</option>
                ))}
              </select>
            </label>
            {instances.map((instance) => {
              const included = stack.members.includes(instance.id);
              const usedElsewhere = stacks.some(
                (other) => other.id !== stack.id && other.members.includes(instance.id),
              );
              return (
                <label key={instance.id}>
                  <input
                    type="checkbox"
                    aria-label={`${stack.name} · ${instance.id}`}
                    checked={included}
                    disabled={
                      usedElsewhere ||
                      (included
                        ? stack.members.length <= 2 || slots >= 4
                        : stack.members.length >= 4)
                    }
                    onChange={(event) => {
                      const members = event.target.checked
                        ? [...stack.members, instance.id]
                        : stack.members.filter((id) => id !== instance.id);
                      emit(
                        instances,
                        stacks.map((item) =>
                          item.id === stack.id
                            ? {
                                ...item,
                                members,
                                activeID: members.includes(item.activeID)
                                  ? item.activeID
                                  : members[0],
                              }
                            : item,
                        ),
                      );
                    }}
                  />
                  {instance.id}
                </label>
              );
            })}
            <button
              type="button"
              disabled={index === 0}
              onClick={() => {
                const next = cloneStacks(stacks);
                [next[index - 1], next[index]] = [next[index], next[index - 1]];
                emit(instances, next);
              }}
            >
              ↑
            </button>
            <button
              type="button"
              disabled={index === stacks.length - 1}
              onClick={() => {
                const next = cloneStacks(stacks);
                [next[index], next[index + 1]] = [next[index + 1], next[index]];
                emit(instances, next);
              }}
            >
              ↓
            </button>
            <button
              type="button"
              disabled={slots + stack.members.length - 1 > 4}
              onClick={() =>
                emit(
                  instances,
                  stacks.filter((item) => item.id !== stack.id),
                )
              }
            >
              {tr(c.dissolve)}
            </button>
          </article>
        ))}
      </section>
    </section>
  );
}

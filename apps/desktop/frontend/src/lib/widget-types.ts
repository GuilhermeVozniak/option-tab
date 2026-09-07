export interface WidgetLease {
  controllerEpoch: number;
  displayUUID: string;
  session: number;
  profileID: string;
  instanceID: string;
  digest: string;
  admissionEpoch: number;
  revision: number;
}

export type Lease = WidgetLease;

export interface RenderNode {
  key: string;
  kind: "row" | "column" | "text" | "icon" | "progress" | "sparkline" | "button";
  text?: string;
  status: string;
  progress?: number;
  history?: number[];
  assetToken?: string;
  actionToken?: string;
  children?: RenderNode[];
}

export interface InstanceState {
  lease: WidgetLease;
  status: string;
  reason?: string;
  root: RenderNode;
}

export interface ActionOption {
  token: string;
  label: string;
}

export interface NumberRange {
  min: number;
  max: number;
  step: number;
}

export interface ActionOptions {
  options: ActionOption[];
  range?: NumberRange;
}

export interface WidgetActions {
  options: (lease: WidgetLease, actionToken: string) => Promise<ActionOptions>;
  perform: (
    lease: WidgetLease,
    actionToken: string,
    optionToken: string,
    value: number | null,
  ) => Promise<unknown>;
  asset: (lease: WidgetLease, assetToken: string) => Promise<string>;
}

export type WidgetLocalized = Record<string, string>;

export interface WidgetSetting {
  id: string;
  type: "choice" | "timezone" | "number" | "boolean";
  name: WidgetLocalized;
  defaultText?: string;
  defaultNumber?: number;
  defaultBool?: boolean;
  options?: string[];
  min?: number;
  max?: number;
}

export interface WidgetCatalogDescriptor {
  packageID: string;
  digest: string;
  version: string;
  name: WidgetLocalized;
  description: WidgetLocalized;
  requiredCapabilities: string[];
  optionalCapabilities: string[];
  settings: WidgetSetting[];
  builtin: boolean;
}

export type WidgetValue = { text: string } | { number: number } | { boolean: boolean };

export interface WidgetInstanceConfig {
  id: string;
  packageID: string;
  digest: string;
  enabled: boolean;
  grants: string[];
  settings?: Record<string, WidgetValue>;
}

export interface WidgetStackConfig {
  id: string;
  name: string;
  members: string[];
  activeID: string;
}

export interface LauncherWidgetChoice {
  id: string;
  name: WidgetLocalized;
}

export interface LauncherWidgetSlot {
  id: string;
  stackID?: string;
  name: WidgetLocalized;
  members: LauncherWidgetChoice[];
  selectedID: string;
  status: string;
  state?: InstanceState;
}

export interface LauncherWidgetState {
  epoch: number;
  displayUUID: string;
  session: number;
  profileID: string;
  revision: number;
  visible: boolean;
  slots: LauncherWidgetSlot[];
}

export interface WidgetPackageStatus {
  available: boolean;
  busy: boolean;
  reason: string;
}

export interface WidgetPackageReview {
  token: string;
  sourceName: string;
  package: WidgetCatalogDescriptor;
  expiresAt: string;
  alreadyInstalled: boolean;
}

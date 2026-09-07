import { Events } from "@wailsio/runtime";
import {
  ActivateLauncherItem,
  CancelWidgetPackageReview,
  GetLauncherAppChoices,
  GetLauncherState,
  GetLauncherStatus,
  GetLauncherWidgets,
  GetWidgetActionOptions,
  GetWidgetAsset,
  GetWidgetCatalog,
  GetWidgetPackageStatus,
  InstallReviewedWidget,
  PerformWidgetAction,
  RemoveWidgetPackage,
  ReviewLocalWidgetPackage,
  SelectLauncherWidget,
  UseNativeDock,
} from "../../bindings/option-tab/app.js";
import type { LauncherAppChoice, LauncherPresentation, LauncherStatus } from "./types";
import type {
  ActionOptions,
  LauncherWidgetState,
  WidgetCatalogDescriptor,
  WidgetLease,
  WidgetPackageReview,
  WidgetPackageStatus,
} from "./widget-types";

export const launcher = {
  state: (session: number) =>
    GetLauncherState(session) as unknown as Promise<LauncherPresentation | null>,
  status: () => GetLauncherStatus() as Promise<LauncherStatus>,
  appChoices: () => GetLauncherAppChoices() as Promise<LauncherAppChoice[]>,
  activate: (
    epoch: number,
    displayUUID: string,
    session: number,
    revision: number,
    itemID: string,
  ) => ActivateLauncherItem(epoch, displayUUID, session, revision, itemID),
  useNativeDock: () => UseNativeDock(),
  catalog: () => GetWidgetCatalog() as Promise<WidgetCatalogDescriptor[]>,
  widgets: (session: number) => GetLauncherWidgets(session) as Promise<LauncherWidgetState>,
  widgetOptions: (lease: WidgetLease, token: string) =>
    GetWidgetActionOptions(lease, token) as Promise<ActionOptions>,
  widgetPerform: (
    lease: WidgetLease,
    actionToken: string,
    optionToken: string,
    value: number | null,
  ) => PerformWidgetAction(lease, actionToken, optionToken, value),
  widgetAsset: (lease: WidgetLease, token: string) => GetWidgetAsset(lease, token),
  selectWidget: (
    epoch: number,
    displayUUID: string,
    session: number,
    profileID: string,
    stackID: string,
    instanceID: string,
  ) => SelectLauncherWidget(epoch, displayUUID, session, profileID, stackID, instanceID),
  packageStatus: () => GetWidgetPackageStatus() as Promise<WidgetPackageStatus>,
  reviewPackage: () => ReviewLocalWidgetPackage() as Promise<WidgetPackageReview>,
  installPackage: (token: string) =>
    InstallReviewedWidget(token) as Promise<WidgetCatalogDescriptor>,
  cancelPackageReview: (token: string) => CancelWidgetPackageReview(token),
  removePackage: (digest: string) => RemoveWidgetPackage(digest),
};
export const onLauncherState = (handler: (state: LauncherPresentation) => void) =>
  Events.On("launcher:state", (event) => handler(event.data as LauncherPresentation));
export const onLauncherStatus = (handler: (status: LauncherStatus) => void) =>
  Events.On("launcher:status", (event) => handler(event.data as LauncherStatus));
export const onLauncherWidgets = (handler: (state: LauncherWidgetState) => void) =>
  Events.On("launcher:widgets", (event) => handler(event.data as LauncherWidgetState));
export const onWidgetPackageStatus = (handler: (status: WidgetPackageStatus) => void) =>
  Events.On("widgets:packages", (event) => handler(event.data as WidgetPackageStatus));

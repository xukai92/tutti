import type { ServiceRegistry } from "@tutti-os/infra/di";
import type {
  TuttidClient,
  TuttidEventStreamClient
} from "@tutti-os/client-tuttid-ts";
import type {
  DesktopHostFilesApi,
  DesktopRuntimeApi,
  DesktopHostWorkspaceApi
} from "@preload/types";
import type { IReporterService } from "../../analytics/services/reporterService.interface.ts";
import { createDesktopWorkspaceAppCenterGateway } from "./internal/adapters/desktopWorkspaceAppCenterGateway.ts";
import {
  configureDaemonBaseUrlResolver,
  createDaemonBaseUrlCache
} from "./internal/workspaceAppLaunchUrl.ts";
import { WorkspaceAppCenterService } from "./internal/workspaceAppCenterService.ts";
import { WorkspaceAppSurfaceHost } from "./internal/workspaceAppSurfaceHost.ts";
import {
  IWorkspaceAppCenterService,
  type IWorkspaceAppCenterService as WorkspaceAppCenterServiceInterface
} from "./workspaceAppCenterService.interface";
import { IWorkspaceAppSurfaceHost } from "./workspaceAppSurfaceHost.interface.ts";

export interface WorkspaceAppCenterServiceRegistrationInput {
  eventStreamClient: TuttidEventStreamClient;
  hostFilesApi: Pick<
    DesktopHostFilesApi,
    | "openExternal"
    | "revealInFolder"
    | "selectAppArchive"
    | "selectAppArchiveExportPath"
    | "selectDirectory"
    | "selectAppIconImage"
  >;
  hostWorkspaceApi: Pick<DesktopHostWorkspaceApi, "openWorkspaceAppFolder">;
  tuttidClient: TuttidClient;
  reporterService?: Pick<IReporterService, "trackEvents">;
  runtimeApi: Pick<DesktopRuntimeApi, "logRendererDiagnostic"> &
    Partial<Pick<DesktopRuntimeApi, "getBackendConfig">>;
}

export function registerWorkspaceAppCenterServices(
  registry: ServiceRegistry,
  input: WorkspaceAppCenterServiceRegistrationInput
): WorkspaceAppCenterServiceInterface {
  const getBackendConfig = input.runtimeApi.getBackendConfig;
  if (getBackendConfig) {
    const baseUrlCache = createDaemonBaseUrlCache(() => getBackendConfig());
    configureDaemonBaseUrlResolver(baseUrlCache.resolve);
  }
  const surfaceHost = new WorkspaceAppSurfaceHost();
  const service = new WorkspaceAppCenterService({
    eventStreamClient: input.eventStreamClient,
    gateway: createDesktopWorkspaceAppCenterGateway(input.tuttidClient),
    hostFilesApi: input.hostFilesApi,
    hostWorkspaceApi: input.hostWorkspaceApi,
    tuttidClient: input.tuttidClient,
    reporterService: input.reporterService,
    runtimeApi: input.runtimeApi,
    surfaceHost
  });
  registry.registerInstance(IWorkspaceAppCenterService, service);
  registry.registerInstance(IWorkspaceAppSurfaceHost, surfaceHost);
  return service;
}

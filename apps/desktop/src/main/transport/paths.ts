import { randomBytes } from "node:crypto";
import { join } from "node:path";
import { resolveDesktopDefaultsFromEnv } from "../defaults.ts";
import { resolveRemoteDaemonConfig } from "./remoteMode.ts";

export interface DesktopDaemonEndpoint {
  accessToken: string;
  boundAddr: string | null;
  listenerInfoPath: string;
  pidPath: string;
  requestedAddr: string;
}

export interface DesktopTerminalStreamUrlInput {
  afterSeq?: number;
  sessionId: string;
  workspaceId: string;
}

export function resolveDesktopDaemonEndpoint(): DesktopDaemonEndpoint {
  const defaults = resolveDesktopDefaultsFromEnv();

  // Remote mode: the daemon lives on another machine. Pre-bind the endpoint to
  // the remote base URL and use the shared access token so every HTTP/WS request
  // is routed there. No local process is spawned or discovered, so the listener
  // info / PID paths are irrelevant but kept populated for logging shape.
  const remote = resolveRemoteDaemonConfig();
  if (remote) {
    return {
      accessToken: remote.accessToken,
      boundAddr: remote.baseUrl,
      listenerInfoPath: defaults.state.tuttidListenerInfoPath,
      pidPath: defaults.state.tuttidPIDPath,
      requestedAddr: remote.baseUrl
    };
  }

  const requestedAddr = process.env.TUTTID_ADDR?.trim() || "127.0.0.1:0";

  return {
    accessToken: randomBytes(32).toString("base64url"),
    boundAddr: null,
    listenerInfoPath: defaults.state.tuttidListenerInfoPath,
    pidPath: defaults.state.tuttidPIDPath,
    requestedAddr
  };
}

const appProxyPathPattern = /\/v1\/workspaces\/[^/]+\/apps\/[^/]+\/proxy(\/|$)/;

/**
 * Returns the `Authorization` header value to attach to a workspace-app webview
 * request destined for the daemon, so the daemon's bearer auth is satisfied.
 *
 * A request qualifies when it targets the daemon origin AND either:
 *  - its own path is an app-proxy path (the initial document load), or
 *  - its Referer is an app-proxy page (root-absolute asset/API requests the app
 *    makes, e.g. "/assets/x.js" or a runtime fetch("/api/..."), which the daemon
 *    routes back to the app by Referer).
 *
 * Returns null for anything else so unrelated traffic is untouched. Safe to call
 * before the endpoint is bound (returns null).
 */
export function resolveDaemonAppProxyAuthHeader(
  endpoint: DesktopDaemonEndpoint,
  requestUrl: string,
  refererUrl?: string
): string | null {
  if (!endpoint.boundAddr || !endpoint.accessToken) {
    return null;
  }
  let base: URL;
  let target: URL;
  try {
    base = new URL(resolveDesktopDaemonBaseUrl(endpoint));
    target = new URL(requestUrl);
  } catch {
    return null;
  }
  if (target.origin !== base.origin) {
    return null;
  }
  const targetIsProxyPath = appProxyPathPattern.test(target.pathname);
  const refererIsProxyPage = isAppProxyReferer(refererUrl, base.origin);
  if (!targetIsProxyPath && !refererIsProxyPage) {
    return null;
  }
  return `Bearer ${endpoint.accessToken}`;
}

function isAppProxyReferer(
  refererUrl: string | undefined,
  daemonOrigin: string
): boolean {
  const trimmed = refererUrl?.trim() ?? "";
  if (trimmed === "") {
    return false;
  }
  try {
    const referer = new URL(trimmed);
    return (
      referer.origin === daemonOrigin &&
      appProxyPathPattern.test(referer.pathname)
    );
  } catch {
    return false;
  }
}

export function resolveDesktopDaemonBaseUrl(
  endpoint: DesktopDaemonEndpoint
): string {
  if (!endpoint.boundAddr) {
    throw new Error("Desktop daemon endpoint is not ready yet.");
  }

  return toBaseUrl(endpoint.boundAddr);
}

export function resolveDesktopTerminalStreamUrl(
  endpoint: DesktopDaemonEndpoint,
  input: DesktopTerminalStreamUrlInput
): string {
  const url = createDesktopWebSocketUrl(
    endpoint,
    `/v1/workspaces/${encodeURIComponent(input.workspaceId)}/terminals/${encodeURIComponent(input.sessionId)}/ws`
  );
  if (input.afterSeq !== undefined) {
    url.searchParams.set("afterSeq", String(input.afterSeq));
  }
  return url.toString();
}

export function resolveDesktopBusinessEventStreamUrl(
  endpoint: DesktopDaemonEndpoint
): string {
  return createDesktopWebSocketUrl(endpoint, "/v1/events/ws").toString();
}

export function resolveDesktopLogsDir(): string {
  return resolveDesktopDefaultsFromEnv().state.logsDir;
}

export function resolveBrowserNodeAutomationListenerInfoPath(): string {
  return join(
    resolveDesktopDefaultsFromEnv().state.runDir,
    "browser-node-automation.json"
  );
}

function toBaseUrl(addr: string): string {
  if (addr.startsWith("http://") || addr.startsWith("https://")) {
    return addr.replace(/\/+$/, "");
  }

  return `http://${addr}`.replace(/\/+$/, "");
}

function createDesktopWebSocketUrl(
  endpoint: DesktopDaemonEndpoint,
  pathname: string
): URL {
  const url = new URL(pathname, resolveDesktopDaemonBaseUrl(endpoint));
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.searchParams.set("access_token", endpoint.accessToken);
  return url;
}

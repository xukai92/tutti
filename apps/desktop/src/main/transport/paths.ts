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

/**
 * Returns the `Authorization` header value to attach to a workspace-app webview
 * request destined for the daemon, so the daemon's bearer auth is satisfied.
 *
 * This is installed only on a dedicated app webview session partition
 * (persist:tutti-app:<ws>:<app>), which carries a single app's traffic. So any
 * request from it to the daemon origin belongs to that app's proxied surface —
 * the initial document, its assets, its /api/* XHRs, and its WebSocket handshake
 * — and all of them must carry the daemon token. Attaching to every
 * daemon-origin request (rather than sniffing the path or Referer) is what makes
 * this robust: browsers omit or truncate Referer for many sub-resource and WS
 * requests, which previously caused intermittent 401s on CSS/XHR/WS.
 *
 * Requests to any other origin (the app's own externally-hosted resources, CDNs)
 * are left untouched. Returns null before the endpoint is bound.
 */
export function resolveDaemonAppProxyAuthHeader(
  endpoint: DesktopDaemonEndpoint,
  requestUrl: string
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
  return `Bearer ${endpoint.accessToken}`;
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

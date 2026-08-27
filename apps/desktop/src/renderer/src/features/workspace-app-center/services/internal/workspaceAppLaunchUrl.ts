// In remote mode the daemon advertises an app's launch URL as a daemon-relative
// reverse-proxy path (e.g. "/v1/workspaces/<ws>/apps/<app>/proxy/") instead of a
// direct http://127.0.0.1:<port> URL, because the app's loopback port is not
// reachable from the client machine. The webview needs an absolute URL, so we
// join the relative path against the daemon base origin the client already uses.
//
// A resolver returns the daemon base URL (or null until it is known). Absolute
// launch URLs (the local-daemon case) pass through untouched.

export type DaemonBaseUrlResolver = () => string | null;

export function absolutizeWorkspaceAppLaunchUrl(
  launchUrl: string | null | undefined,
  resolveDaemonBaseUrl: DaemonBaseUrlResolver
): string | null {
  const trimmed = launchUrl?.trim() ?? "";
  if (trimmed === "") {
    return null;
  }
  // Already absolute (local daemon, or an externally-hosted app): leave as-is.
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(trimmed)) {
    return trimmed;
  }
  // Only join root-relative daemon paths; anything else is left untouched so we
  // never fabricate an origin for an unexpected value.
  if (!trimmed.startsWith("/")) {
    return trimmed;
  }
  const baseUrl = resolveDaemonBaseUrl();
  if (!baseUrl) {
    return trimmed;
  }
  try {
    return new URL(trimmed, baseUrl).toString();
  } catch {
    return trimmed;
  }
}

// Module-level resolver used by the app-center mappers, which are pure standalone
// functions with many call sites. Configured once at service registration via
// configureDaemonBaseUrlResolver. Defaults to "unknown" (relative URLs pass
// through) so behavior is unchanged until explicitly configured.
let sharedDaemonBaseUrlResolver: DaemonBaseUrlResolver = () => null;

export function configureDaemonBaseUrlResolver(
  resolver: DaemonBaseUrlResolver
): void {
  sharedDaemonBaseUrlResolver = resolver;
}

export function absolutizeWorkspaceAppLaunchUrlWithSharedResolver(
  launchUrl: string | null | undefined
): string | null {
  return absolutizeWorkspaceAppLaunchUrl(
    launchUrl,
    sharedDaemonBaseUrlResolver
  );
}

// createDaemonBaseUrlCache resolves the daemon base URL once (async) and exposes
// a synchronous getter for the pure mappers that need it. Until the first
// resolution completes, the getter returns null and relative URLs pass through
// unchanged; a subsequent app-state update re-runs the mapper with the cached
// value.
export function createDaemonBaseUrlCache(
  getBackendConfig: () => Promise<{ baseUrl: string }>
): { resolve: DaemonBaseUrlResolver } {
  let cached: string | null = null;
  void Promise.resolve()
    .then(() => getBackendConfig())
    .then((config) => {
      cached = config.baseUrl;
    })
    .catch(() => {
      cached = null;
    });
  return { resolve: () => cached };
}

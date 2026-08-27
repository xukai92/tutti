// Remote daemon mode lets the desktop client talk to a `tuttid` running on a
// different machine instead of spawning and managing a local child process.
//
// When TUTTID_REMOTE_URL is set, the desktop:
//   - resolves its daemon endpoint to that URL (so every HTTP and WS request is
//     routed there via the existing transport in ./fetch.ts and ./paths.ts), and
//   - skips launching a managed local tuttid (see daemon/tuttidManager.ts).
//
// The remote daemon must be started with a matching TUTTID_ACCESS_TOKEN, which
// the client sends here as the bearer token. Because the token travels over the
// network (including as an `access_token` query param on WebSocket URLs), a
// remote daemon should only ever be reached over TLS (https/wss), e.g. via a
// reverse proxy or SSH tunnel — never raw http over an untrusted network.

export interface RemoteDaemonConfig {
  /**
   * Base URL of the remote daemon, always normalized to include an explicit
   * scheme and no trailing slash (e.g. "https://tutti.example.com:4545").
   */
  baseUrl: string;
  /** Bearer token matching the remote daemon's TUTTID_ACCESS_TOKEN. */
  accessToken: string;
}

/**
 * Resolve the remote daemon configuration from the environment, or null when
 * remote mode is disabled (the default: the desktop manages a local daemon).
 *
 * Throws when TUTTID_REMOTE_URL is set but the configuration is incomplete or
 * malformed, so misconfiguration surfaces loudly at startup rather than as an
 * opaque connection failure later.
 */
export function resolveRemoteDaemonConfig(
  env: NodeJS.ProcessEnv = process.env
): RemoteDaemonConfig | null {
  const rawUrl = env.TUTTID_REMOTE_URL?.trim();
  if (!rawUrl) {
    return null;
  }

  const baseUrl = normalizeRemoteBaseUrl(rawUrl);
  if (!baseUrl) {
    throw new Error(
      `TUTTID_REMOTE_URL is not a valid http(s) URL: ${JSON.stringify(rawUrl)}`
    );
  }

  const accessToken = env.TUTTID_REMOTE_ACCESS_TOKEN?.trim();
  if (!accessToken) {
    throw new Error(
      "TUTTID_REMOTE_URL is set but TUTTID_REMOTE_ACCESS_TOKEN is missing; " +
        "set it to the remote daemon's TUTTID_ACCESS_TOKEN."
    );
  }

  return { accessToken, baseUrl };
}

export function isRemoteDaemonModeEnabled(
  env: NodeJS.ProcessEnv = process.env
): boolean {
  return Boolean(env.TUTTID_REMOTE_URL?.trim());
}

/**
 * Normalize a user-supplied remote URL into a scheme-qualified base URL with no
 * trailing slash. A bare "host:port" defaults to https. Returns null when the
 * value cannot be parsed as an http(s) URL.
 */
export function normalizeRemoteBaseUrl(rawUrl: string): string | null {
  const trimmed = rawUrl.trim();
  if (trimmed === "") {
    return null;
  }

  // Reject an explicit non-http scheme (e.g. ftp://, ws://) rather than silently
  // coercing it to https. A value with no "scheme://" prefix defaults to https.
  const explicitScheme = /^([a-z][a-z0-9+.-]*):\/\//i.exec(trimmed);
  if (explicitScheme) {
    const scheme = explicitScheme[1]?.toLowerCase();
    if (scheme !== "http" && scheme !== "https") {
      return null;
    }
  }

  const candidate = explicitScheme ? trimmed : `https://${trimmed}`;

  let parsed: URL;
  try {
    parsed = new URL(candidate);
  } catch {
    return null;
  }

  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    return null;
  }

  return parsed.toString().replace(/\/+$/, "");
}

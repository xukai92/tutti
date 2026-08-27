import assert from "node:assert/strict";
import test from "node:test";
import {
  isRemoteDaemonModeEnabled,
  normalizeRemoteBaseUrl,
  resolveRemoteDaemonConfig
} from "./remoteMode.ts";

test("resolveRemoteDaemonConfig returns null when remote mode is disabled", () => {
  assert.equal(resolveRemoteDaemonConfig({}), null);
  assert.equal(resolveRemoteDaemonConfig({ TUTTID_REMOTE_URL: "   " }), null);
});

test("resolveRemoteDaemonConfig resolves url and token", () => {
  assert.deepEqual(
    resolveRemoteDaemonConfig({
      TUTTID_REMOTE_URL: "https://tutti.example.com:4545/",
      TUTTID_REMOTE_ACCESS_TOKEN: "secret-token"
    }),
    {
      accessToken: "secret-token",
      baseUrl: "https://tutti.example.com:4545"
    }
  );
});

test("resolveRemoteDaemonConfig defaults a bare host to https", () => {
  const config = resolveRemoteDaemonConfig({
    TUTTID_REMOTE_URL: "tutti.example.com:4545",
    TUTTID_REMOTE_ACCESS_TOKEN: "secret-token"
  });
  assert.equal(config?.baseUrl, "https://tutti.example.com:4545");
});

test("resolveRemoteDaemonConfig throws when token is missing", () => {
  assert.throws(
    () =>
      resolveRemoteDaemonConfig({
        TUTTID_REMOTE_URL: "https://tutti.example.com:4545"
      }),
    /TUTTID_REMOTE_ACCESS_TOKEN is missing/
  );
});

test("resolveRemoteDaemonConfig throws on an unparseable url", () => {
  assert.throws(
    () =>
      resolveRemoteDaemonConfig({
        TUTTID_REMOTE_URL: "http://",
        TUTTID_REMOTE_ACCESS_TOKEN: "secret-token"
      }),
    /not a valid http\(s\) URL/
  );
});

test("resolveRemoteDaemonConfig rejects non-http schemes", () => {
  assert.throws(
    () =>
      resolveRemoteDaemonConfig({
        TUTTID_REMOTE_URL: "ftp://tutti.example.com",
        TUTTID_REMOTE_ACCESS_TOKEN: "secret-token"
      }),
    /not a valid http\(s\) URL/
  );
});

test("isRemoteDaemonModeEnabled reflects TUTTID_REMOTE_URL presence", () => {
  assert.equal(isRemoteDaemonModeEnabled({}), false);
  assert.equal(isRemoteDaemonModeEnabled({ TUTTID_REMOTE_URL: "" }), false);
  assert.equal(
    isRemoteDaemonModeEnabled({ TUTTID_REMOTE_URL: "https://x.example" }),
    true
  );
});

test("normalizeRemoteBaseUrl strips trailing slashes and preserves scheme", () => {
  assert.equal(
    normalizeRemoteBaseUrl("http://localhost:4545///"),
    "http://localhost:4545"
  );
  assert.equal(
    normalizeRemoteBaseUrl("https://tutti.example.com"),
    "https://tutti.example.com/".replace(/\/+$/, "")
  );
  assert.equal(normalizeRemoteBaseUrl(""), null);
});

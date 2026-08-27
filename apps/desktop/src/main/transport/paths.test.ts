import assert from "node:assert/strict";
import test from "node:test";
import {
  resolveDaemonAppProxyAuthHeader,
  type DesktopDaemonEndpoint
} from "./paths.ts";

function endpoint(
  overrides: Partial<DesktopDaemonEndpoint> = {}
): DesktopDaemonEndpoint {
  return {
    accessToken: "secret-token",
    boundAddr: "http://127.0.0.1:4545",
    listenerInfoPath: "/tmp/listener.json",
    pidPath: "/tmp/pid",
    requestedAddr: "http://127.0.0.1:4545",
    ...overrides
  };
}

test("resolveDaemonAppProxyAuthHeader attaches bearer for a daemon proxy request", () => {
  assert.equal(
    resolveDaemonAppProxyAuthHeader(
      endpoint(),
      "http://127.0.0.1:4545/v1/workspaces/ws-1/apps/app-1/proxy/assets/x.js"
    ),
    "Bearer secret-token"
  );
});

test("resolveDaemonAppProxyAuthHeader attaches bearer to any daemon-origin path", () => {
  for (const path of [
    "/assets/index-abc.js",
    "/api/projects/x",
    "/api/ws",
    "/v1/workspaces/ws-1/apps/app-1/proxy/"
  ]) {
    assert.equal(
      resolveDaemonAppProxyAuthHeader(
        endpoint(),
        `http://127.0.0.1:4545${path}`
      ),
      "Bearer secret-token",
      `expected bearer for ${path}`
    );
  }
});

test("resolveDaemonAppProxyAuthHeader ignores other origins", () => {
  assert.equal(
    resolveDaemonAppProxyAuthHeader(
      endpoint(),
      "http://example.com/assets/index-abc.js"
    ),
    null
  );
});

test("resolveDaemonAppProxyAuthHeader returns null before the endpoint is bound", () => {
  assert.equal(
    resolveDaemonAppProxyAuthHeader(
      endpoint({ boundAddr: null }),
      "http://127.0.0.1:4545/v1/workspaces/ws-1/apps/app-1/proxy/"
    ),
    null
  );
});

test("resolveDaemonAppProxyAuthHeader tolerates malformed request urls", () => {
  assert.equal(resolveDaemonAppProxyAuthHeader(endpoint(), "not a url"), null);
});

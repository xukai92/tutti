import assert from "node:assert/strict";
import test from "node:test";
import { absolutizeWorkspaceAppLaunchUrl } from "./workspaceAppLaunchUrl.ts";

const base = () => "http://127.0.0.1:4545";

test("absolutizeWorkspaceAppLaunchUrl joins a relative proxy path against the daemon base", () => {
  assert.equal(
    absolutizeWorkspaceAppLaunchUrl(
      "/v1/workspaces/ws-1/apps/app-1/proxy/",
      base
    ),
    "http://127.0.0.1:4545/v1/workspaces/ws-1/apps/app-1/proxy/"
  );
});

test("absolutizeWorkspaceAppLaunchUrl leaves absolute URLs untouched", () => {
  assert.equal(
    absolutizeWorkspaceAppLaunchUrl("http://127.0.0.1:51234", base),
    "http://127.0.0.1:51234"
  );
});

test("absolutizeWorkspaceAppLaunchUrl passes through when base is unknown", () => {
  assert.equal(
    absolutizeWorkspaceAppLaunchUrl("/v1/x/proxy/", () => null),
    "/v1/x/proxy/"
  );
});

test("absolutizeWorkspaceAppLaunchUrl returns null for empty input", () => {
  assert.equal(absolutizeWorkspaceAppLaunchUrl("", base), null);
  assert.equal(absolutizeWorkspaceAppLaunchUrl(null, base), null);
  assert.equal(absolutizeWorkspaceAppLaunchUrl(undefined, base), null);
});

test("absolutizeWorkspaceAppLaunchUrl leaves non-rooted relative values untouched", () => {
  assert.equal(
    absolutizeWorkspaceAppLaunchUrl("about:blank", base),
    "about:blank"
  );
});

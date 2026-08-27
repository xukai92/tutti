package workspace

import (
	"net/url"
	"os"
	"strings"
)

// AppProxyEnabledEnv, when truthy, makes the daemon advertise apps to clients
// via a daemon-relative reverse-proxy path instead of the app's raw
// http://127.0.0.1:<port> URL. This is required when the desktop client runs on
// a different machine than the daemon (remote mode): the loopback URL is not
// reachable from the client, but the daemon's proxy endpoint is.
const AppProxyEnabledEnv = "TUTTID_APP_PROXY_ENABLED"

// appProxyEnabled reports whether launch URLs should be projected as daemon
// proxy paths. Split out for testability.
func appProxyEnabled(getenv func(string) string) bool {
	value := strings.TrimSpace(getenv(AppProxyEnabledEnv))
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// ExternalLaunchURL returns the launch URL that should be advertised to clients
// for a running app. In proxy mode it returns a daemon-relative path
// (/v1/workspaces/{ws}/apps/{app}/proxy/) that the client resolves against the
// daemon base URL; otherwise it returns the app's own loopback URL unchanged.
//
// A nil rawLaunchURL (app not running / no URL yet) is returned unchanged.
func ExternalLaunchURL(
	workspaceID string,
	appID string,
	rawLaunchURL *string,
) *string {
	return externalLaunchURL(workspaceID, appID, rawLaunchURL, os.Getenv)
}

func externalLaunchURL(
	workspaceID string,
	appID string,
	rawLaunchURL *string,
	getenv func(string) string,
) *string {
	if rawLaunchURL == nil || strings.TrimSpace(*rawLaunchURL) == "" {
		return rawLaunchURL
	}
	if !appProxyEnabled(getenv) {
		return rawLaunchURL
	}
	proxied := "/v1/workspaces/" + url.PathEscape(strings.TrimSpace(workspaceID)) +
		"/apps/" + url.PathEscape(strings.TrimSpace(appID)) + "/proxy/"
	return &proxied
}

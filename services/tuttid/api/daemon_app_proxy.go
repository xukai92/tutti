package api

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	workspacebiz "github.com/tutti-os/tutti/services/tuttid/biz/workspace"
	tuttitypes "github.com/tutti-os/tutti/services/tuttid/types"
)

// AppProxyPathSegment is the path segment under an app that reverse-proxies to
// the app's own loopback HTTP server. When the desktop client runs on a
// different machine than the daemon (remote mode), an app's raw
// http://127.0.0.1:<port> launch URL is unreachable from the client, so the
// client instead loads /v1/workspaces/{ws}/apps/{app}/proxy/... and the daemon
// forwards it to the app process over loopback.
const AppProxyPathSegment = "proxy"

// appProxyPrefix builds the path prefix that ProxyWorkspaceApp serves and strips
// before forwarding to the app.
func appProxyPrefix(workspaceID string, appID string) string {
	return "/v1/workspaces/" + url.PathEscape(workspaceID) +
		"/apps/" + url.PathEscape(appID) + "/" + AppProxyPathSegment
}

// ProxyWorkspaceApp on daemonRoutes delegates to the DaemonAPI implementation so
// the reverse proxy participates in the Routes interface alongside the generated
// handlers.
func (routes daemonRoutes) ProxyWorkspaceApp(w http.ResponseWriter, r *http.Request) {
	routes.api.ProxyWorkspaceApp(w, r)
}

// ProxyWorkspaceApp reverse-proxies a request to a running workspace app's local
// HTTP server. The app keeps binding to loopback only; this handler is the sole
// network-reachable entry point, so it inherits the daemon's bearer auth.
func (api DaemonAPI) ProxyWorkspaceApp(w http.ResponseWriter, r *http.Request) {
	if api.AppCenterService == nil {
		tuttitypes.WriteError(
			w,
			http.StatusServiceUnavailable,
			tuttitypes.ErrorCodeServiceUnavailable,
			"workspace_app_service_unavailable",
			"workspace app service is unavailable",
		)
		return
	}

	workspaceID := strings.TrimSpace(r.PathValue("workspaceID"))
	appID := strings.TrimSpace(r.PathValue("appID"))
	if workspaceID == "" || appID == "" {
		tuttitypes.WriteError(
			w,
			http.StatusBadRequest,
			tuttitypes.ErrorCodeInvalidRequest,
			"invalid_path",
			"workspace id and app id are required",
		)
		return
	}

	target, ok := api.resolveRunningAppTarget(r.Context(), workspaceID, appID)
	if !ok {
		tuttitypes.WriteError(
			w,
			http.StatusNotFound,
			"not_found",
			"app_not_running",
			"workspace app is not running",
		)
		return
	}

	prefix := appProxyPrefix(workspaceID, appID)
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Strip the daemon proxy prefix so the app sees a root-relative path.
		trimmed := strings.TrimPrefix(req.URL.Path, prefix)
		if trimmed == "" {
			trimmed = "/"
		}
		if !strings.HasPrefix(trimmed, "/") {
			trimmed = "/" + trimmed
		}
		req.URL.Path = trimmed
		req.URL.RawPath = ""
		// The app authenticates callers with its own server token; the daemon's
		// bearer header must not leak downstream.
		req.Header.Del("Authorization")
		req.Host = target.Host
	}

	proxy.ServeHTTP(w, r)
}

// resolveRunningAppTarget returns the loopback base URL of a running app.
func (api DaemonAPI) resolveRunningAppTarget(
	ctx context.Context,
	workspaceID string,
	appID string,
) (*url.URL, bool) {
	apps, err := api.AppCenterService.List(ctx, workspaceID)
	if err != nil {
		return nil, false
	}
	for _, app := range apps {
		if app.Package.AppID != appID {
			continue
		}
		if app.Runtime.Status != workspacebiz.AppRuntimeStatusRunning {
			return nil, false
		}
		if app.Runtime.LaunchURL == nil {
			return nil, false
		}
		target, err := url.Parse(strings.TrimSpace(*app.Runtime.LaunchURL))
		if err != nil || target.Host == "" {
			return nil, false
		}
		return target, true
	}
	return nil, false
}

package api

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
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

// appProxyPrefixPattern extracts {workspaceID, appID} from an app proxy path.
// It matches both ".../proxy" and ".../proxy/<rest>".
var appProxyPrefixPattern = regexp.MustCompile(
	`/v1/workspaces/([^/]+)/apps/([^/]+)/` + AppProxyPathSegment + `(?:/|$)`,
)

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

// ProxyWorkspaceAppByReferer on daemonRoutes delegates to the DaemonAPI
// implementation.
func (routes daemonRoutes) ProxyWorkspaceAppByReferer(w http.ResponseWriter, r *http.Request) {
	routes.api.ProxyWorkspaceAppByReferer(w, r)
}

// ProxyWorkspaceApp reverse-proxies a request to a running workspace app's local
// HTTP server. The app keeps binding to loopback only; this handler is the sole
// network-reachable entry point, so it inherits the daemon's bearer auth.
//
// It serves the app-proxy prefix (/v1/workspaces/{ws}/apps/{app}/proxy/...) and
// strips that prefix before forwarding, so the app sees a root-relative path.
func (api DaemonAPI) ProxyWorkspaceApp(w http.ResponseWriter, r *http.Request) {
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
	api.proxyToApp(w, r, workspaceID, appID, appProxyPrefix(workspaceID, appID))
}

// ProxyWorkspaceAppByReferer handles requests an app makes to root-absolute
// paths (e.g. "/assets/index.js", "/styles.css", or a runtime fetch("/api/..."))
// that do not carry the app proxy prefix. Tutti apps are built to be served from
// the origin root, so their asset and API URLs are root-absolute and would
// otherwise miss the app entirely once the app is served under a path prefix.
//
// The originating app is recovered from the Referer header, which for any
// in-document request points at the app's proxy URL. The request path is
// forwarded to the app unchanged (no prefix to strip). Requests without a
// recoverable app-proxy Referer are passed to the fallback handler, so genuine
// unknown routes still 404 normally.
func (api DaemonAPI) ProxyWorkspaceAppByReferer(w http.ResponseWriter, r *http.Request) {
	workspaceID, appID, ok := appIdentityFromReferer(r.Header.Get("Referer"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	// No prefix to strip: the app asked for a root-absolute path and expects it
	// verbatim.
	api.proxyToApp(w, r, workspaceID, appID, "")
}

func appIdentityFromReferer(referer string) (string, string, bool) {
	referer = strings.TrimSpace(referer)
	if referer == "" {
		return "", "", false
	}
	parsed, err := url.Parse(referer)
	if err != nil {
		return "", "", false
	}
	match := appProxyPrefixPattern.FindStringSubmatch(parsed.Path)
	if match == nil {
		return "", "", false
	}
	workspaceID, err := url.PathUnescape(match[1])
	if err != nil {
		return "", "", false
	}
	appID, err := url.PathUnescape(match[2])
	if err != nil {
		return "", "", false
	}
	if workspaceID == "" || appID == "" {
		return "", "", false
	}
	return workspaceID, appID, true
}

// proxyToApp reverse-proxies r to the running app identified by (workspaceID,
// appID). When stripPrefix is non-empty it is removed from the request path
// before forwarding.
func (api DaemonAPI) proxyToApp(
	w http.ResponseWriter,
	r *http.Request,
	workspaceID string,
	appID string,
	stripPrefix string,
) {
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

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		path := req.URL.Path
		if stripPrefix != "" {
			path = strings.TrimPrefix(path, stripPrefix)
		}
		if path == "" {
			path = "/"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		req.URL.Path = path
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

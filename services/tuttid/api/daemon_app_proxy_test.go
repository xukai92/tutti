package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	workspacebiz "github.com/tutti-os/tutti/services/tuttid/biz/workspace"
)

type proxyListAppCenterService struct {
	stubAppCenterService
	apps []workspacebiz.WorkspaceApp
}

func (s proxyListAppCenterService) List(context.Context, string) ([]workspacebiz.WorkspaceApp, error) {
	return s.apps, nil
}

func newProxyRequest(t *testing.T, path string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	// The real mux populates PathValue; emulate it for the handler under test.
	req.SetPathValue("workspaceID", "ws-1")
	req.SetPathValue("appID", "app-1")
	return req
}

func TestProxyWorkspaceAppForwardsAndStripsPrefix(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Header.Get("Authorization") != "" {
			t.Errorf("Authorization header must not be forwarded to app")
		}
		_, _ = io.WriteString(w, "app-body")
	}))
	defer upstream.Close()

	launchURL := upstream.URL
	api := DaemonAPI{AppCenterService: proxyListAppCenterService{
		apps: []workspacebiz.WorkspaceApp{{
			Package: workspacebiz.AppPackage{AppID: "app-1"},
			Runtime: workspacebiz.AppRuntimeState{
				Status:    workspacebiz.AppRuntimeStatusRunning,
				LaunchURL: &launchURL,
			},
		}},
	}}

	rec := httptest.NewRecorder()
	req := newProxyRequest(t, "/v1/workspaces/ws-1/apps/app-1/proxy/assets/x.js")
	req.Header.Set("Authorization", "Bearer secret")
	api.ProxyWorkspaceApp(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotPath != "/assets/x.js" {
		t.Fatalf("expected upstream path /assets/x.js, got %q", gotPath)
	}
	if rec.Body.String() != "app-body" {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}
}

func TestProxyWorkspaceAppRootPath(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	}))
	defer upstream.Close()

	launchURL := upstream.URL
	api := DaemonAPI{AppCenterService: proxyListAppCenterService{
		apps: []workspacebiz.WorkspaceApp{{
			Package: workspacebiz.AppPackage{AppID: "app-1"},
			Runtime: workspacebiz.AppRuntimeState{
				Status:    workspacebiz.AppRuntimeStatusRunning,
				LaunchURL: &launchURL,
			},
		}},
	}}

	rec := httptest.NewRecorder()
	api.ProxyWorkspaceApp(rec, newProxyRequest(t, "/v1/workspaces/ws-1/apps/app-1/proxy"))
	if gotPath != "/" {
		t.Fatalf("expected upstream root path /, got %q", gotPath)
	}
}

func TestProxyWorkspaceAppNotRunning(t *testing.T) {
	api := DaemonAPI{AppCenterService: proxyListAppCenterService{
		apps: []workspacebiz.WorkspaceApp{{
			Package: workspacebiz.AppPackage{AppID: "app-1"},
			Runtime: workspacebiz.AppRuntimeState{Status: workspacebiz.AppRuntimeStatusStarting},
		}},
	}}
	rec := httptest.NewRecorder()
	api.ProxyWorkspaceApp(rec, newProxyRequest(t, "/v1/workspaces/ws-1/apps/app-1/proxy/"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestProxyWorkspaceAppByRefererForwardsRootPath(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	}))
	defer upstream.Close()

	launchURL := upstream.URL
	api := DaemonAPI{AppCenterService: proxyListAppCenterService{
		apps: []workspacebiz.WorkspaceApp{{
			Package: workspacebiz.AppPackage{AppID: "app-1"},
			Runtime: workspacebiz.AppRuntimeState{
				Status:    workspacebiz.AppRuntimeStatusRunning,
				LaunchURL: &launchURL,
			},
		}},
	}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/index-abc.js", nil)
	req.Header.Set(
		"Referer",
		"http://127.0.0.1:4545/v1/workspaces/ws-1/apps/app-1/proxy/",
	)
	api.ProxyWorkspaceAppByReferer(rec, req)

	if gotPath != "/assets/index-abc.js" {
		t.Fatalf("expected root path forwarded verbatim, got %q", gotPath)
	}
}

func TestProxyWorkspaceAppSetsRouteCookie(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer upstream.Close()

	launchURL := upstream.URL
	api := DaemonAPI{AppCenterService: proxyListAppCenterService{
		apps: []workspacebiz.WorkspaceApp{{
			Package: workspacebiz.AppPackage{AppID: "app-1"},
			Runtime: workspacebiz.AppRuntimeState{
				Status:    workspacebiz.AppRuntimeStatusRunning,
				LaunchURL: &launchURL,
			},
		}},
	}}

	rec := httptest.NewRecorder()
	api.ProxyWorkspaceApp(rec, newProxyRequest(t, "/v1/workspaces/ws-1/apps/app-1/proxy/"))

	var routeCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == appProxyRouteCookie {
			routeCookie = c
		}
	}
	if routeCookie == nil {
		t.Fatalf("expected %s cookie to be set", appProxyRouteCookie)
	}
	ws, app, ok := decodeAppRouteCookie(routeCookie.Value)
	if !ok || ws != "ws-1" || app != "app-1" {
		t.Fatalf("decodeAppRouteCookie(%q) = (%q,%q,%v)", routeCookie.Value, ws, app, ok)
	}
}

func TestProxyWorkspaceAppByRefererRoutesViaCookieWithoutReferer(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	}))
	defer upstream.Close()

	launchURL := upstream.URL
	api := DaemonAPI{AppCenterService: proxyListAppCenterService{
		apps: []workspacebiz.WorkspaceApp{{
			Package: workspacebiz.AppPackage{AppID: "app-1"},
			Runtime: workspacebiz.AppRuntimeState{
				Status:    workspacebiz.AppRuntimeStatusRunning,
				LaunchURL: &launchURL,
			},
		}},
	}}

	rec := httptest.NewRecorder()
	// No Referer — only the route cookie. This is the CSS/asset failure mode.
	req := httptest.NewRequest(http.MethodGet, "/assets/index-abc.css", nil)
	req.AddCookie(&http.Cookie{
		Name:  appProxyRouteCookie,
		Value: encodeAppRouteCookie("ws-1", "app-1"),
	})
	api.ProxyWorkspaceAppByReferer(rec, req)

	if gotPath != "/assets/index-abc.css" {
		t.Fatalf("expected cookie-routed path, got %q", gotPath)
	}
}

func TestProxyWorkspaceAppByRefererWithoutProxyRefererIs404(t *testing.T) {
	api := DaemonAPI{AppCenterService: proxyListAppCenterService{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/index-abc.js", nil)
	req.Header.Set("Referer", "http://127.0.0.1:4545/some/other/page")
	api.ProxyWorkspaceAppByReferer(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestProxyWorkspaceAppServiceUnavailable(t *testing.T) {
	api := DaemonAPI{}
	rec := httptest.NewRecorder()
	api.ProxyWorkspaceApp(rec, newProxyRequest(t, "/v1/workspaces/ws-1/apps/app-1/proxy/"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
